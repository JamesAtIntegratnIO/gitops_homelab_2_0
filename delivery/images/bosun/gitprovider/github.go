package gitprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// GitHub implements Provider against the REST API.
//
// Deliberately hand-rolled over net/http rather than pulling a client library:
// the surface used here is five endpoints, and a vendored SDK would be by far
// the largest dependency in a service whose whole point is to be small enough
// to audit.
type GitHub struct {
	// APIBase allows GitHub Enterprise. Defaults to the public API.
	APIBase string
	Owner   string
	Repo    string
	Token   string
	// AuthorName and AuthorEmail identify the agent's commits. Worth setting
	// to something recognisable -- these commits land on a bot branch and a
	// reviewer should be able to tell instantly who wrote them.
	AuthorName  string
	AuthorEmail string
	HTTP        *http.Client
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) base() string {
	if g.APIBase != "" {
		return strings.TrimRight(g.APIBase, "/")
	}
	return "https://api.github.com"
}

func (g *GitHub) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (g *GitHub) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base()+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, snippet(payload))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(payload, out)
}

func (g *GitHub) repoPath(suffix string) string {
	return fmt.Sprintf("/repos/%s/%s%s", g.Owner, g.Repo, suffix)
}

func (g *GitHub) GetPullRequest(ctx context.Context, number int) (*PullRequest, error) {
	var pr struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		HTML   string `json:"html_url"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			SHA string `json:"sha"`
		} `json:"base"`
		User   struct{ Login string }  `json:"user"`
		Labels []struct{ Name string } `json:"labels"`
	}
	if err := g.do(ctx, http.MethodGet, g.repoPath(fmt.Sprintf("/pulls/%d", number)), nil, &pr); err != nil {
		return nil, err
	}
	out := &PullRequest{
		Number: pr.Number, Title: pr.Title, Body: pr.Body,
		Branch: pr.Head.Ref, HeadSHA: pr.Head.SHA, BaseSHA: pr.Base.SHA,
		Author: pr.User.Login, URL: pr.HTML,
	}
	for _, l := range pr.Labels {
		out.Labels = append(out.Labels, l.Name)
	}
	return out, nil
}

func (g *GitHub) ListComments(ctx context.Context, number int) ([]Comment, error) {
	var raw []struct {
		Body string                 `json:"body"`
		User struct{ Login string } `json:"user"`
	}
	if err := g.do(ctx, http.MethodGet,
		g.repoPath(fmt.Sprintf("/issues/%d/comments?per_page=100", number)), nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Comment, 0, len(raw))
	for _, c := range raw {
		out = append(out, Comment{Author: c.User.Login, Body: c.Body})
	}
	return out, nil
}

// CheckStatus reports the aggregate state of one named check.
//
// Both surfaces are consulted: check runs (GitHub Actions) and legacy commit
// statuses, because a repository can use either and a gate reported through
// the one you did not look at is indistinguishable from no gate at all.
func (g *GitHub) CheckStatus(ctx context.Context, sha, checkName string) (CheckState, error) {
	var runs struct {
		CheckRuns []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := g.do(ctx, http.MethodGet,
		g.repoPath(fmt.Sprintf("/commits/%s/check-runs?per_page=100", sha)), nil, &runs); err == nil {
		for _, r := range runs.CheckRuns {
			if r.Name != checkName {
				continue
			}
			if r.Status != "completed" {
				return CheckPending, nil
			}
			switch r.Conclusion {
			case "success", "neutral", "skipped":
				return CheckSuccess, nil
			default:
				return CheckFailure, nil
			}
		}
	}

	var statuses []struct {
		Context string `json:"context"`
		State   string `json:"state"`
	}
	if err := g.do(ctx, http.MethodGet,
		g.repoPath(fmt.Sprintf("/commits/%s/statuses?per_page=100", sha)), nil, &statuses); err != nil {
		return CheckMissing, err
	}
	for _, s := range statuses {
		if s.Context != checkName {
			continue
		}
		switch s.State {
		case "success":
			return CheckSuccess, nil
		case "pending":
			return CheckPending, nil
		default:
			return CheckFailure, nil
		}
	}
	return CheckMissing, nil
}

func (g *GitHub) Comment(ctx context.Context, number int, body string) error {
	return g.do(ctx, http.MethodPost,
		g.repoPath(fmt.Sprintf("/issues/%d/comments", number)),
		map[string]string{"body": body}, nil)
}

func (g *GitHub) AddLabel(ctx context.Context, number int, label string) error {
	return g.do(ctx, http.MethodPost,
		g.repoPath(fmt.Sprintf("/issues/%d/labels", number)),
		map[string][]string{"labels": {label}}, nil)
}

// PushFix commits and pushes the working tree onto the pull request's branch.
//
// Uses git over HTTPS with the token in the remote URL rather than the API's
// blob/tree endpoints: it is a handful of commands, it produces an ordinary
// commit, and it works identically for any host once the URL changes.
//
// The push target is always the pull request's own branch. There is no code
// path here that writes to the default branch.
func (g *GitHub) PushFix(ctx context.Context, pr *PullRequest, root, message string) error {
	if pr.Branch == "" {
		return fmt.Errorf("pull request has no head branch")
	}
	name := g.AuthorName
	if name == "" {
		name = "bosun"
	}
	email := g.AuthorEmail
	if email == "" {
		email = "bosun@users.noreply.github.com"
	}
	remote := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", g.Token, g.Owner, g.Repo)

	steps := [][]string{
		{"git", "-C", root, "config", "user.name", name},
		{"git", "-C", root, "config", "user.email", email},
		{"git", "-C", root, "add", "-A"},
		{"git", "-C", root, "commit", "-m", message},
		{"git", "-C", root, "push", remote, "HEAD:" + pr.Branch},
	}
	for _, s := range steps {
		cmd := exec.CommandContext(ctx, s[0], s[1:]...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// Redact the token before this reaches a log or a PR comment.
			msg := strings.ReplaceAll(stderr.String(), g.Token, "***")
			return fmt.Errorf("%s: %w: %s", s[1], err, snippet([]byte(msg)))
		}
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		s = s[:400] + "..."
	}
	return s
}
