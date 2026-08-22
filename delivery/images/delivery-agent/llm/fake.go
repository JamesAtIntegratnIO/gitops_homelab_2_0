package llm

import "context"

// Fake is a Provider that answers with whatever it was given.
//
// Not in a _test.go file because the caller under test is package main: driving
// the triage workflow needs a model seam that returns a chosen verdict, or a
// chosen failure, without a backend behind it.
type Fake struct {
	// Verdict is returned as-is, unvalidated. A test asserting what the agent
	// does with a badly calibrated verdict has to be able to supply one.
	Verdict *Verdict
	// Err stands in for a model that is down, slow or misconfigured -- a case
	// the agent must not let look like a verdict.
	Err error
	// ID is the name reported in logs and PR comments.
	ID string

	// System and User record the last prompt, so a test can assert on the
	// evidence the model was shown -- the same string the applier corroborates
	// version-shaped values against.
	System string
	User   string
	Calls  int
}

func (f *Fake) Classify(_ context.Context, system, user string) (*Verdict, error) {
	f.Calls++
	f.System, f.User = system, user
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Verdict, nil
}

func (f *Fake) Name() string {
	if f.ID == "" {
		return "fake/model"
	}
	return f.ID
}
