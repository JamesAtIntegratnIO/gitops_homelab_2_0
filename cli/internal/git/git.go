package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo provides git operations scoped to a repository.
type Repo struct {
	// Root is the absolute path to the git repository root.
	Root string
}

// DetectRepo finds the git repository root from the given directory (or cwd).
func DetectRepo(dir string) (*Repo, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or any parent): %w", err)
	}

	return &Repo{Root: strings.TrimSpace(out)}, nil
}

// Status returns the short status of the repository.
func (r *Repo) Status() (string, error) {
	return runGit(r.Root, "status", "--short")
}

// IsClean returns true if the working tree has no uncommitted changes.
func (r *Repo) IsClean() (bool, error) {
	out, err := r.Status()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// CurrentBranch returns the current branch name.
func (r *Repo) CurrentBranch() (string, error) {
	out, err := runGit(r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Add stages files for commit. Paths are relative to the repo root.
func (r *Repo) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := runGit(r.Root, args...)
	return err
}

// Commit creates a commit with the given message.
func (r *Repo) Commit(message string) error {
	_, err := runGit(r.Root, "commit", "-m", message)
	return err
}

// Push pushes the current branch to the remote.
func (r *Repo) Push(remote string) error {
	if remote == "" {
		remote = "origin"
	}
	branch, err := r.CurrentBranch()
	if err != nil {
		return err
	}
	_, err = runGit(r.Root, "push", remote, branch)
	return err
}

// CommitAndPush is a convenience method that stages, commits, and pushes.
func (r *Repo) CommitAndPush(paths []string, message string) error {
	if err := r.Add(paths...); err != nil {
		return fmt.Errorf("staging files: %w", err)
	}
	if err := r.Commit(message); err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	if err := r.Push(""); err != nil {
		return fmt.Errorf("pushing: %w", err)
	}
	return nil
}

// FormatCommitMessage creates a standardized hctl commit message.
func FormatCommitMessage(action, resource, details string) string {
	msg := fmt.Sprintf("hctl: %s %s", action, resource)
	if details != "" {
		msg += fmt.Sprintf(" (%s)", details)
	}
	return msg
}

// RelPath returns a path relative to the repo root.
func (r *Repo) RelPath(absPath string) (string, error) {
	return filepath.Rel(r.Root, absPath)
}

// ReconcileAnnotation returns the current timestamp string for reconcile annotations.
func ReconcileAnnotation() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
