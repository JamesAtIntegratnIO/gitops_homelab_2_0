package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, args []string) error {
	cfg := config.Default()

	results, err := tui.RunSteps(tui.IconPlay+"  Initializing hctl", []tui.Step{
		{
			Title: "Detecting git repository",
			Run: func() (string, error) {
				repo, err := git.DetectRepo("")
				if err != nil {
					return "Could not detect git repository", nil // non-fatal
				}
				cfg.RepoPath = repo.Root
				branch, _ := repo.CurrentBranch()
				detail := repo.Root
				if branch != "" {
					detail += " (" + branch + ")"
				}
				return detail, nil
			},
		},
		{
			Title: "Checking cluster access",
			Run: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", fmt.Errorf("cannot connect: %w", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				nodes, err := client.ListNodes(ctx)
				if err != nil {
					return "", fmt.Errorf("cannot list nodes: %w", err)
				}
				return fmt.Sprintf("%d nodes reachable", len(nodes)), nil
			},
		},
		{
			Title: "Saving configuration",
			Run: func() (string, error) {
				if err := config.Save(cfg); err != nil {
					return "", err
				}
				return config.ConfigPath(), nil
			},
		},
	})

	if err != nil {
		return hcerrors.NewPlatformError("initializing hctl: %w", err)
	}

	// Check if any steps failed
	for _, r := range results {
		if r.Err != nil {
			return hcerrors.NewPlatformError("init failed at %q: %w", r.Title, r.Err)
		}
	}

	return nil
}
