// Package gitprovider is the git-host seam.
//
// Four methods, chosen because they are what the triage workflow actually
// needs and nothing more. Adding a host is one implementation of this
// interface with no change to the triage logic -- which is the whole reason
// the judgement half lives in the cluster rather than in a CI system's native
// syntax.
package gitprovider

import "context"

// PullRequest is the subset of a pull request the agent reasons about.
type PullRequest struct {
	Number  int
	Title   string
	Body    string
	Branch  string
	BaseSHA string
	HeadSHA string
	Labels  []string
	Author  string
	URL     string
}

// Comment is one comment on a pull request. The agent reads these because the
// gate publishes its report as one -- a comment is the only artifact surface
// every git host has, which keeps the gate's output reachable without a
// provider-specific artifacts API.
type Comment struct {
	Author string
	Body   string
}

// CheckState is the aggregate state of the gate on a commit.
type CheckState string

const (
	CheckPending CheckState = "pending"
	CheckSuccess CheckState = "success"
	CheckFailure CheckState = "failure"
	CheckMissing CheckState = "missing"
)

// Provider is one git host.
type Provider interface {
	// GetPullRequest reads a pull request.
	GetPullRequest(ctx context.Context, number int) (*PullRequest, error)

	// ListComments returns the pull request's comments, oldest first.
	ListComments(ctx context.Context, number int) ([]Comment, error)

	// CheckStatus reports the aggregate state of the named check on a commit.
	CheckStatus(ctx context.Context, sha, checkName string) (CheckState, error)

	// Comment posts a comment.
	Comment(ctx context.Context, number int, body string) error

	// AddLabel adds a label. Labels carry the attempt cap, so this has to be
	// durable across restarts -- which is exactly why the cap is a label
	// rather than in-memory state.
	AddLabel(ctx context.Context, number int, label string) error

	// PushFix commits the working tree at root onto the pull request's branch.
	//
	// Never to the default branch. The agent's entire write surface is the
	// bot's own branch, and the change still has to pass the gate and the
	// merge policy to reach anywhere.
	PushFix(ctx context.Context, pr *PullRequest, root, message string) error

	// Name identifies the provider in logs.
	Name() string
}
