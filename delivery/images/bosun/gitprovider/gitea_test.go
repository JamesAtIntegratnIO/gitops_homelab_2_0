package gitprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Gitea implementation is a port of the GitHub one, so the tests worth
// having are the places the two hosts genuinely differ. A test that only
// proved "GET /pulls/1 returns a PullRequest" would pass against either and
// catch none of the three things that actually bite.

func giteaFor(t *testing.T, h http.Handler) (*Gitea, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Gitea{
		BaseURL: srv.URL, Owner: "org", Repo: "repo",
		Token: "s3cret", HTTP: srv.Client(),
	}, srv
}

func TestGiteaReadsPullRequest(t *testing.T) {
	g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/org/repo/pulls/7" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "token s3cret" {
			t.Errorf("Authorization = %q, want Gitea's `token` form", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"number": 7, "title": "bump", "body": "b", "html_url": "http://x/7",
			"head":   map[string]string{"ref": "kargo/bump", "sha": "deadbeef"},
			"base":   map[string]string{"sha": "cafe"},
			"user":   map[string]string{"login": "kargo"},
			"labels": []map[string]string{{"name": "automated"}},
		})
	}))

	pr, err := g.GetPullRequest(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Branch != "kargo/bump" || pr.HeadSHA != "deadbeef" || pr.BaseSHA != "cafe" {
		t.Fatalf("pull request read wrong: %+v", pr)
	}
	if len(pr.Labels) != 1 || pr.Labels[0] != "automated" {
		t.Fatalf("labels = %v", pr.Labels)
	}
}

// Gitea has no check-runs API. Everything reports as a commit status, so a
// gate that reported as one must be readable -- and a name that matches
// nothing has to be Missing rather than an error, or the agent cannot tell
// "no gate here" from "the host is broken".
func TestGiteaCheckStatusReadsCommitStatuses(t *testing.T) {
	cases := []struct {
		status string
		want   CheckState
	}{
		{"success", CheckSuccess},
		{"pending", CheckPending},
		{"failure", CheckFailure},
		{"error", CheckFailure},
		{"warning", CheckFailure},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode([]map[string]string{
					{"context": "other", "status": "success"},
					{"context": "gate", "status": tc.status},
				})
			}))
			got, err := g.CheckStatus(context.Background(), "sha1", "gate")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGiteaCheckStatusMissingIsNotAnError(t *testing.T) {
	g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{{"context": "something-else", "status": "success"}})
	}))
	got, err := g.CheckStatus(context.Background(), "sha1", "gate")
	if err != nil {
		t.Fatal(err)
	}
	if got != CheckMissing {
		t.Fatalf("state = %q, want %q", got, CheckMissing)
	}
}

// A commit accumulates one status per CI re-run, newest first. Reading past
// the first match would report a result that has already been superseded --
// the agent would act on a red gate that is now green, or the reverse.
func TestGiteaCheckStatusTakesTheNewest(t *testing.T) {
	g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{
			{"context": "gate", "status": "success"},
			{"context": "gate", "status": "failure"},
		})
	}))
	got, _ := g.CheckStatus(context.Background(), "sha1", "gate")
	if got != CheckSuccess {
		t.Fatalf("state = %q, want the newest status (%q)", got, CheckSuccess)
	}
}

// The attempt cap is a label, so a label that silently fails to attach makes
// the agent retry forever. Gitea wants numeric IDs, which means the name has
// to be resolved first.
func TestGiteaAddLabelResolvesNameToID(t *testing.T) {
	var posted map[string][]int64
	g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/org/repo/labels":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 11, "name": "other"},
				{"id": 42, "name": "agent/attempted"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/org/repo/issues/7/labels":
			json.NewDecoder(r.Body).Decode(&posted)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))

	if err := g.AddLabel(context.Background(), 7, "agent/attempted"); err != nil {
		t.Fatal(err)
	}
	if len(posted["labels"]) != 1 || posted["labels"][0] != 42 {
		t.Fatalf("posted %v, want the resolved id 42", posted)
	}
}

// A fresh repository has none of the agent's labels. Failing here would make
// the cap fail open on exactly the repositories most likely to loop.
func TestGiteaAddLabelCreatesAMissingLabel(t *testing.T) {
	created := false
	var posted map[string][]int64
	g, _ := giteaFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/org/repo/labels":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/org/repo/labels":
			created = true
			json.NewEncoder(w).Encode(map[string]any{"id": 99, "name": "agent/attempted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/org/repo/issues/7/labels":
			json.NewDecoder(r.Body).Decode(&posted)
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))

	if err := g.AddLabel(context.Background(), 7, "agent/attempted"); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("label was not created")
	}
	if len(posted["labels"]) != 1 || posted["labels"][0] != 99 {
		t.Fatalf("posted %v, want the created id 99", posted)
	}
}

// An HTTP failure carries the request URL, and PushFix's remote carries the
// token. Nothing that reaches a log or a pull request comment may contain it.
func TestGiteaRedactsTheToken(t *testing.T) {
	if got := redactErr("fatal: https://u:s3cret@host/x.git denied", "s3cret"); got != "fatal: https://u:***@host/x.git denied" {
		t.Fatalf("redaction left the token in: %q", got)
	}
}

func TestGiteaPushFixRefusesWithoutABranch(t *testing.T) {
	g := &Gitea{BaseURL: "http://x", Owner: "o", Repo: "r"}
	if err := g.PushFix(context.Background(), &PullRequest{}, t.TempDir(), "m"); err == nil {
		t.Fatal("pushing to an empty branch must fail: the default branch is not a fallback")
	}
}

func TestGiteaName(t *testing.T) {
	g := &Gitea{}
	if g.Name() != "gitea" {
		t.Fatalf("Name() = %q", g.Name())
	}
	var _ Provider = (*Gitea)(nil)
}
