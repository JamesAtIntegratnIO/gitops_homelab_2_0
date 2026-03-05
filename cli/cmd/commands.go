package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
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

func runDiagnose(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	var result *platform.DiagnosticResult

	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	_, err = tui.Spin("Diagnosing "+name, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err = platform.DiagnoseVCluster(ctx, client, cfg.Platform.PlatformNamespace, name)
		return "", err
	})
	if err != nil {
		return hcerrors.NewPlatformError("diagnosing %s: %w", name, err)
	}

	// Structured output or bundle export
	if tui.IsStructured() || bundlePath != "" {
		bundle := map[string]interface{}{
			"resource":   name,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"steps":      result.Steps,
		}
		if bundlePath != "" {
			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return hcerrors.NewPlatformError("marshaling diagnostic bundle: %w", err)
			}
			if writeErr := os.WriteFile(bundlePath, data, 0o644); writeErr != nil {
				return hcerrors.NewPlatformError("writing bundle: %w", writeErr)
			}
			fmt.Printf("%s Diagnostic bundle written to %s\n",
				tui.SuccessStyle.Render(tui.IconCheck), bundlePath)
			if !tui.IsStructured() {
				return nil // don't print text output if we wrote a bundle
			}
		}
		return tui.RenderOutput(bundle, "")
	}

	fmt.Printf("\n  %s\n", name)
	for i, step := range result.Steps {
		isLast := i == len(result.Steps)-1
		fmt.Println(tui.TreeNode(
			fmt.Sprintf("%-15s", step.Name),
			tui.DiagIcon(int(step.Status)),
			step.Message,
			isLast,
		))
		if step.Details != "" {
			indent := "  │   "
			if isLast {
				indent = "      "
			}
			fmt.Printf("%s%s\n", indent, tui.DimStyle.Render(step.Details))
		}
	}
	fmt.Println()

	return nil
}

func runReconcile(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.SetManualReconciliationLabel(ctx, kube.VClusterOrchestratorV2GVR, cfg.Platform.PlatformNamespace, name)
	if err != nil {
		return hcerrors.NewPlatformError("setting reconciliation label: %w", err)
	}

	fmt.Printf("  %s Set kratix.io/manual-reconciliation=true on %s\n", tui.SuccessStyle.Render(tui.IconCheck), name)
	return nil
}

func runContext(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	fmt.Println()
	fmt.Println(tui.TitleStyle.Render("  Platform Context"))
	fmt.Println()
	fmt.Println(tui.KeyValue("Repo", tui.ValueOrMuted(cfg.RepoPath, "(not set)")))
	fmt.Println(tui.KeyValue("Git Mode", cfg.GitMode))
	fmt.Println(tui.KeyValue("ArgoCD", cfg.ArgocdURL))
	fmt.Println(tui.KeyValue("Domain", cfg.Platform.Domain))
	fmt.Println(tui.KeyValue("Namespace", cfg.Platform.PlatformNamespace))
	fmt.Println(tui.KeyValue("Config", tui.MutedStyle.Render(config.ConfigPath())))
	fmt.Println()

	return nil
}
