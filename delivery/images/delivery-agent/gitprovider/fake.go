package gitprovider

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Fake is an in-memory Provider.
//
// It lives outside _test.go because the workflow it exists to exercise lives in
// package main: the triage tests need a git host they can set up and assert on,
// and this is the only way to give them one without a token and a network.
//
// One triage run at a time -- nothing here is synchronised, because the
// workflow it stands in for is sequential and a mutex would force every
// assertion through an accessor.
type Fake struct {
	// PR is what GetPullRequest returns. Nil makes the read fail, which is the
	// only way this provider can fail that way.
	PR *PullRequest
	// Comments is the pull request's history, oldest first. The gate's report
	// belongs here, marked, or triage has nothing to hand the model.
	Comments []Comment
	Check    CheckState
	CheckErr error
	PushErr  error

	// Posted, Labelled and Pushes are what the run did. Everything the agent is
	// allowed to change about a pull request lands in one of the three, so a
	// test asserting all three has covered the whole write surface.
	Posted   []string
	Labelled []string
	Pushes   []Push
}

// Push is one recorded PushFix, including the working tree as it stood. The
// tree is snapshotted rather than committed: what matters to a test is which
// bytes the agent was about to publish, not that git could store them.
type Push struct {
	Branch  string
	Message string
	Tree    map[string]string
}

func (f *Fake) Name() string { return "fake" }

func (f *Fake) GetPullRequest(_ context.Context, number int) (*PullRequest, error) {
	if f.PR == nil {
		return nil, fmt.Errorf("no pull request %d", number)
	}
	// A copy, so a label added during the run does not appear retroactively on
	// the pull request the run is already holding.
	out := *f.PR
	out.Labels = append([]string(nil), f.PR.Labels...)
	return &out, nil
}

func (f *Fake) ListComments(_ context.Context, _ int) ([]Comment, error) {
	return append([]Comment(nil), f.Comments...), nil
}

func (f *Fake) CheckStatus(_ context.Context, _, _ string) (CheckState, error) {
	if f.CheckErr != nil {
		return CheckMissing, f.CheckErr
	}
	if f.Check == "" {
		return CheckMissing, nil
	}
	return f.Check, nil
}

func (f *Fake) Comment(_ context.Context, _ int, body string) error {
	f.Posted = append(f.Posted, body)
	if f.PR != nil {
		f.Comments = append(f.Comments, Comment{Author: f.Name(), Body: body})
	}
	return nil
}

func (f *Fake) AddLabel(_ context.Context, _ int, label string) error {
	f.Labelled = append(f.Labelled, label)
	if f.PR != nil {
		f.PR.Labels = append(f.PR.Labels, label)
	}
	return nil
}

func (f *Fake) PushFix(_ context.Context, pr *PullRequest, root, message string) error {
	if f.PushErr != nil {
		return f.PushErr
	}
	if pr.Branch == "" {
		return fmt.Errorf("pull request has no head branch")
	}
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		return err
	}
	f.Pushes = append(f.Pushes, Push{Branch: pr.Branch, Message: message, Tree: tree})
	return nil
}
