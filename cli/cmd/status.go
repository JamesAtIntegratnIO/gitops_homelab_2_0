package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	// Structured output mode: collect all status as JSON/YAML
	if tui.IsStructured() {
		if watchFlag {
			return runStatusWatch(cmd.Context(), cfg)
		}
		return runStatusOnce(cfg)
	}

	// Create a single shared kube client for all dashboard sections.
	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ns := cfg.Platform.PlatformNamespace

	return tui.RunDashboard(tui.IconPlay+" Platform Status", []tui.DashboardSection{
		{Title: "Nodes", Load: func() (string, error) { return loadNodesSection(client) }},
		{Title: "ArgoCD", Load: func() (string, error) { return loadArgoCDSection(client) }},
		{Title: "Promises", Load: func() (string, error) { return loadPromisesSection(client) }},
		{Title: "vClusters", Load: func() (string, error) { return loadVClustersSection(client, ns) }},
		{Title: "Workloads", Load: func() (string, error) { return loadWorkloadsSection(client, ns) }},
		{Title: "Addons", Load: func() (string, error) { return loadAddonsSection(client, ns) }},
	})
}

// runStatusOnce collects platform status and prints it once in structured format.
func runStatusOnce(cfg *config.Config) error {
	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps, err := platform.CollectPlatformStatus(ctx, client, cfg.Platform.PlatformNamespace)
	if err != nil {
		return hcerrors.NewPlatformError("collecting platform status: %w", err)
	}
	return tui.RenderOutput(ps, "")
}

// runStatusWatch continuously polls and prints platform status in structured format.
// It respects context cancellation and terminates after 3 consecutive poll failures.
func runStatusWatch(ctx context.Context, cfg *config.Config) error {
	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	const maxConsecutiveErrors = 3
	consecutiveErrors := 0

	// Print immediately, then on each tick
	for {
		pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		ps, err := platform.CollectPlatformStatus(pollCtx, client, cfg.Platform.PlatformNamespace)
		cancel()
		if err != nil {
			consecutiveErrors++
			fmt.Fprintf(os.Stderr, "error (%d/%d): %v\n", consecutiveErrors, maxConsecutiveErrors, err)
			if consecutiveErrors >= maxConsecutiveErrors {
				return hcerrors.NewPlatformError("status watch terminated after %d consecutive failures: %w", maxConsecutiveErrors, err)
			}
		} else {
			consecutiveErrors = 0
			if err := tui.RenderOutput(ps, ""); err != nil {
				fmt.Fprintf(os.Stderr, "render error: %v\n", err)
			}
		}

		select {
		case <-ticker.C:
			// continue polling
		case <-ctx.Done():
			return nil
		}
	}
}
