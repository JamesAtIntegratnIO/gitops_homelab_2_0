package vcluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var appFilter string
	var force bool

	cmd := &cobra.Command{
		Use:   "sync [name]",
		Short: "Force-sync ArgoCD apps for a vCluster",
		Long: `Clears stale operation states and triggers fresh sync on ArgoCD applications
targeting the specified vCluster. Handles dependency ordering — CRD apps are
synced first, then dependent applications.

By default, only syncs apps that are in a Failed/Error state. Use --force to
sync all apps regardless of current state.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			apps, err := client.ListArgoAppsForCluster(ctx, "argocd", name)
			if err != nil {
				return fmt.Errorf("listing apps for cluster %s: %w", name, err)
			}

			if len(apps) == 0 {
				fmt.Printf("%s No ArgoCD applications found targeting cluster %q\n",
					tui.DimStyle.Render("○"), name)
				return nil
			}

			// Filter to specific app if requested
			if appFilter != "" {
				var filtered []kube.ArgoAppInfo
				for _, app := range apps {
					if app.Name == appFilter {
						filtered = append(filtered, app)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("app %q not found targeting cluster %s", appFilter, name)
				}
				apps = filtered
			}

			// Separate into CRD/prerequisite apps and regular apps for dependency ordering
			var crdApps, regularApps []kube.ArgoAppInfo
			for _, app := range apps {
				if isCRDApp(app.Name) {
					crdApps = append(crdApps, app)
				} else {
					regularApps = append(regularApps, app)
				}
			}

			cleared, synced, skipped, errors := 0, 0, 0, 0

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(fmt.Sprintf("Syncing %s (%d apps)", name, len(apps))))

			// Phase 1: CRD apps first
			if len(crdApps) > 0 {
				fmt.Printf("  %s\n", tui.WarningStyle.Render("Phase 1: CRD Prerequisites"))
				for _, app := range crdApps {
					c, s, sk, e := syncApp(ctx, client, app, force)
					cleared += c
					synced += s
					skipped += sk
					errors += e
				}
				// Brief pause between phases to let CRDs register
				if len(regularApps) > 0 {
					fmt.Printf("  %s\n", tui.DimStyle.Render("  Waiting for CRDs to register..."))
					time.Sleep(3 * time.Second)
				}
			}

			// Phase 2: Regular apps
			if len(regularApps) > 0 {
				if len(crdApps) > 0 {
					fmt.Printf("\n  %s\n", tui.WarningStyle.Render("Phase 2: Applications"))
				}
				for _, app := range regularApps {
					c, s, sk, e := syncApp(ctx, client, app, force)
					cleared += c
					synced += s
					skipped += sk
					errors += e
				}
			}

			fmt.Printf("\n  %s cleared: %d, synced: %d, skipped: %d, errors: %d\n\n",
				tui.SuccessStyle.Render("Done."), cleared, synced, skipped, errors)

			return nil
		},
	}

	cmd.Flags().StringVar(&appFilter, "app", "", "sync only a specific app by name")
	cmd.Flags().BoolVar(&force, "force", false, "sync all apps, not just failed/errored ones")

	return cmd
}

// isCRDApp returns true if the app name suggests it deploys CRDs that other apps depend on.
func isCRDApp(name string) bool {
	lowerName := strings.ToLower(name)
	crdPatterns := []string{
		"crd",
		"gateway-api",
	}
	for _, pattern := range crdPatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	return false
}

// syncApp clears stale operation and triggers sync for a single app.
// Returns (cleared, synced, skipped, errors) counts.
func syncApp(ctx context.Context, client *kube.Client, app kube.ArgoAppInfo, force bool) (int, int, int, int) {
	needsSync := force ||
		app.OpPhase == "Failed" ||
		app.OpPhase == "Error" ||
		app.SyncStatus == "OutOfSync" ||
		app.SyncStatus == "Unknown" ||
		app.SyncStatus == ""

	if !needsSync {
		fmt.Printf("    %s %s %s\n",
			tui.DimStyle.Render("○"),
			app.Name,
			tui.DimStyle.Render("(already synced)"))
		return 0, 0, 1, 0
	}

	cleared, synced, errors := 0, 0, 0

	// Clear stale operation state if present
	if app.OpPhase == "Failed" || app.OpPhase == "Error" {
		err := client.ClearArgoAppOperationState(ctx, "argocd", app.Name)
		if err != nil {
			fmt.Printf("    %s %s: clear operation failed: %s\n",
				tui.ErrorStyle.Render("✗"), app.Name, err)
			return 0, 0, 0, 1
		}
		cleared++
		fmt.Printf("    %s %s: cleared %s operation (retries: %d)\n",
			tui.WarningStyle.Render("↺"), app.Name, app.OpPhase, app.RetryCount)
	}

	// Trigger fresh sync
	err := client.TriggerArgoAppSync(ctx, "argocd", app.Name)
	if err != nil {
		// If the error is "another operation is already in progress", that's OK — auto-sync may have kicked in
		if strings.Contains(err.Error(), "another operation is already in progress") {
			fmt.Printf("    %s %s: sync already in progress\n",
				tui.SuccessStyle.Render("⟳"), app.Name)
			return cleared, 1, 0, 0
		}
		fmt.Printf("    %s %s: sync failed: %s\n",
			tui.ErrorStyle.Render("✗"), app.Name, err)
		return cleared, 0, 0, 1
	}

	synced++
	fmt.Printf("    %s %s: sync triggered\n",
		tui.SuccessStyle.Render("✓"), app.Name)

	return cleared, synced, 0, errors
}
