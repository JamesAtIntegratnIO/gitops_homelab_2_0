package git

import (
	"fmt"
)

// GitResult describes what happened after HandleGitWorkflow ran.
type GitResult int

const (
	// GitCommitted means changes were committed and pushed.
	GitCommitted GitResult = iota
	// GitCommittedLocal means changes were committed locally only (generate mode).
	GitCommittedLocal
	// GitStaged means files were staged but user declined to commit.
	GitStaged
	// GitSkipped means git was skipped entirely (generate mode or no repo detected).
	GitSkipped
	// GitNoRepo means no git repository was detected.
	GitNoRepo
)

// UI abstracts the user-interaction methods that HandleGitWorkflow needs,
// keeping the git package free of TUI dependencies.
type UI interface {
	// Confirm asks a yes/no question and returns the answer.
	Confirm(prompt string) (bool, error)
	// PrintSuccess prints a success-styled message.
	PrintSuccess(msg string)
	// PrintDim prints a dim/muted message.
	PrintDim(msg string)
}

// WorkflowOpts configures HandleGitWorkflow behavior.
type WorkflowOpts struct {
	// RepoPath is the working directory or path inside the repo.
	RepoPath string
	// Paths are relative file paths (to repo root) to stage.
	Paths []string
	// Action is the verb for the commit message (e.g. "create vcluster").
	Action string
	// Resource is the resource name for the commit message.
	Resource string
	// Details is optional extra context for the commit message.
	Details string
	// GitMode overrides the config git mode ("auto", "generate", "prompt").
	GitMode string
	// Interactive controls whether prompts are shown.
	Interactive bool
	// ConfirmPrompt overrides the default "Commit and push?" prompt.
	ConfirmPrompt string
	// UI provides user interaction callbacks. Required for "auto", "generate",
	// and "prompt" modes; may be nil for "default"/skip.
	UI UI
}

// HandleGitWorkflow executes the standard git commit/push workflow based on
// the configured gitMode. It consolidates the duplicated 3-way switch pattern
// found across vcluster create, vcluster delete, deploy run, deploy remove,
// addon enable, and addon disable.
//
// Returns the GitResult describing what happened and any error.
func HandleGitWorkflow(opts WorkflowOpts) (GitResult, error) {
	repo, err := DetectRepo(opts.RepoPath)
	if err != nil {
		return GitNoRepo, nil // non-fatal: user can commit manually
	}

	msg := FormatCommitMessage(opts.Action, opts.Resource, opts.Details)
	prompt := opts.ConfirmPrompt
	if prompt == "" {
		prompt = "Commit and push?"
	}

	switch opts.GitMode {
	case "auto":
		if err := repo.CommitAndPush(opts.Paths, msg); err != nil {
			return GitCommitted, fmt.Errorf("git commit/push: %w", err)
		}
		if opts.UI != nil {
			opts.UI.PrintSuccess("Committed and pushed")
		}
		return GitCommitted, nil

	case "generate":
		if err := repo.Add(opts.Paths...); err != nil {
			return GitSkipped, fmt.Errorf("staging files: %w", err)
		}
		if err := repo.Commit(msg); err != nil {
			return GitStaged, fmt.Errorf("committing: %w", err)
		}
		if opts.UI != nil {
			opts.UI.PrintSuccess("Committed (push manually)")
		}
		return GitCommittedLocal, nil

	case "prompt":
		if !opts.Interactive {
			return GitSkipped, nil
		}
		if opts.UI == nil {
			return GitSkipped, nil
		}
		confirmed, confirmErr := opts.UI.Confirm(prompt)
		if confirmErr != nil {
			return GitSkipped, fmt.Errorf("confirming git operation: %w", confirmErr)
		}
		if !confirmed {
			// Best-effort stage so files aren't lost
			if err := repo.Add(opts.Paths...); err != nil {
				return GitSkipped, fmt.Errorf("staging git changes: %w", err)
			}
			opts.UI.PrintDim("  Skipped git commit. Run manually: git add && git commit && git push")
			return GitStaged, nil
		}
		if err := repo.CommitAndPush(opts.Paths, msg); err != nil {
			return GitCommitted, fmt.Errorf("git commit/push: %w", err)
		}
		if opts.UI != nil {
			opts.UI.PrintSuccess("Committed and pushed")
		}
		return GitCommitted, nil

	default:
		if opts.UI != nil {
			opts.UI.PrintDim("  Generated only. Commit and push when ready.")
		}
		return GitSkipped, nil
	}
}


