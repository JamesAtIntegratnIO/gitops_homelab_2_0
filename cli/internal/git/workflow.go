package git

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/tui"
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
		fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render(tui.IconCheck))
		return GitCommitted, nil

	case "generate":
		if err := repo.Add(opts.Paths...); err != nil {
			return GitSkipped, fmt.Errorf("staging files: %w", err)
		}
		if err := repo.Commit(msg); err != nil {
			return GitStaged, fmt.Errorf("committing: %w", err)
		}
		fmt.Printf("%s Committed (push manually)\n", tui.SuccessStyle.Render(tui.IconCheck))
		return GitCommittedLocal, nil

	case "prompt":
		if !opts.Interactive {
			return GitSkipped, nil
		}
		confirmed, _ := tui.Confirm(prompt)
		if !confirmed {
			// Best-effort stage so files aren't lost
			_ = repo.Add(opts.Paths...)
			fmt.Println(tui.DimStyle.Render("  Skipped git commit. Run manually: git add && git commit && git push"))
			return GitStaged, nil
		}
		if err := repo.CommitAndPush(opts.Paths, msg); err != nil {
			return GitCommitted, fmt.Errorf("git commit/push: %w", err)
		}
		fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render(tui.IconCheck))
		return GitCommitted, nil

	default:
		fmt.Println(tui.DimStyle.Render("  Generated only. Commit and push when ready."))
		return GitSkipped, nil
	}
}

// HandleGitWorkflowStep returns a tui.Step for use inside tui.RunSteps.
// If gitMode is "prompt", the prompt is shown *before* calling RunSteps,
// so this should only be used with "auto" or "generate" modes.
func HandleGitWorkflowStep(opts WorkflowOpts) tui.Step {
	var stepTitle string
	switch opts.GitMode {
	case "auto":
		stepTitle = "Committing and pushing"
	case "generate":
		stepTitle = "Committing changes"
	default:
		stepTitle = "Staging files"
	}

	return tui.Step{
		Title: stepTitle,
		Run: func() (string, error) {
			repo, err := DetectRepo(opts.RepoPath)
			if err != nil {
				return "no git repo detected", nil
			}

			msg := FormatCommitMessage(opts.Action, opts.Resource, opts.Details)

			switch opts.GitMode {
			case "auto":
				if err := repo.CommitAndPush(opts.Paths, msg); err != nil {
					return "", err
				}
				return msg, nil
			case "generate":
				if err := repo.Add(opts.Paths...); err != nil {
					return "", err
				}
				if err := repo.Commit(msg); err != nil {
					return "", err
				}
				return msg + " (push manually)", nil
			default:
				_ = repo.Add(opts.Paths...)
				return "staged — commit manually", nil
			}
		},
	}
}
