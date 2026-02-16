package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println(tui.TitleStyle.Render("🏗  hctl init"))
	fmt.Println()

	cfg := config.Default()

	// Detect repo
	repo, err := git.DetectRepo("")
	if err != nil {
		fmt.Println(tui.WarningStyle.Render("⚠  Could not detect git repository from current directory"))
		fmt.Println(tui.DimStyle.Render("  Run hctl init from within the gitops repository, or provide --config"))
	} else {
		cfg.RepoPath = repo.Root
		fmt.Printf("  Repo: %s\n", tui.SuccessStyle.Render(repo.Root))
		branch, _ := repo.CurrentBranch()
		if branch != "" {
			fmt.Printf("  Branch: %s\n", branch)
		}
	}

	// Check kubectl connectivity
	fmt.Print("\n  Checking cluster access... ")
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		fmt.Println(tui.ErrorStyle.Render("✗ " + err.Error()))
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		nodes, err := client.ListNodes(ctx)
		if err != nil {
			fmt.Println(tui.ErrorStyle.Render("✗ " + err.Error()))
		} else {
			fmt.Println(tui.SuccessStyle.Render(fmt.Sprintf("✓ %d nodes", len(nodes))))
		}
	}

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("\n  Config written to %s\n", tui.DimStyle.Render(config.ConfigPath()))
	fmt.Println(tui.SuccessStyle.Render("\n✓ hctl initialized"))

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println(tui.TitleStyle.Render("Platform Status"))
	fmt.Println()

	// Nodes
	fmt.Println(tui.HeaderStyle.Render("Nodes"))
	nodes, err := client.ListNodes(ctx)
	if err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗"), err.Error())
	} else {
		var rows [][]string
		for _, n := range nodes {
			status := tui.StatusIcon(n.Ready)
			rows = append(rows, []string{n.Name, status, n.IP, strings.Join(n.Roles, ","), n.CPU, n.Memory})
		}
		fmt.Println(tui.Table([]string{"NAME", "READY", "IP", "ROLES", "CPU", "MEMORY"}, rows))
	}

	// ArgoCD Applications
	fmt.Println(tui.HeaderStyle.Render("ArgoCD Applications"))
	apps, err := client.ListArgoApps(ctx, "argocd")
	if err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗"), err.Error())
	} else {
		synced, outOfSync, degraded, healthy := 0, 0, 0, 0
		for _, app := range apps {
			syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
			healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
			if syncStatus == "Synced" {
				synced++
			} else {
				outOfSync++
			}
			if healthStatus == "Healthy" {
				healthy++
			} else if healthStatus == "Degraded" {
				degraded++
			}
		}
		fmt.Printf("  Total: %d  |  Synced: %s  |  OutOfSync: %s  |  Healthy: %s  |  Degraded: %s\n\n",
			len(apps),
			tui.SuccessStyle.Render(fmt.Sprintf("%d", synced)),
			tui.WarningStyle.Render(fmt.Sprintf("%d", outOfSync)),
			tui.SuccessStyle.Render(fmt.Sprintf("%d", healthy)),
			tui.ErrorStyle.Render(fmt.Sprintf("%d", degraded)),
		)

		// List unhealthy apps
		if outOfSync > 0 || degraded > 0 {
			fmt.Println(tui.WarningStyle.Render("  Unhealthy applications:"))
			for _, app := range apps {
				syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
				healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
				if syncStatus != "Synced" || healthStatus != "Healthy" {
					fmt.Printf("    %s  sync=%s health=%s\n", app.GetName(), syncStatus, healthStatus)
				}
			}
			fmt.Println()
		}
	}

	// Kratix Promises
	fmt.Println(tui.HeaderStyle.Render("Kratix Promises"))
	promises, err := client.ListPromises(ctx)
	if err != nil {
		fmt.Printf("  %s %s\n", tui.WarningStyle.Render("⚠"), err.Error())
	} else {
		var rows [][]string
		for _, p := range promises {
			status := "Unknown"
			conditions, _, _ := platform.UnstructuredNestedSlice(p.Object, "status", "conditions")
			for _, c := range conditions {
				if cm, ok := c.(map[string]interface{}); ok {
					if cm["type"] == "Available" {
						if cm["status"] == "True" {
							status = tui.SuccessStyle.Render("Available")
						} else {
							status = tui.ErrorStyle.Render("Unavailable")
						}
					}
				}
			}
			rows = append(rows, []string{p.GetName(), status})
		}
		fmt.Println(tui.Table([]string{"PROMISE", "STATUS"}, rows))
	}

	// vClusters
	fmt.Println(tui.HeaderStyle.Render("vClusters"))
	vclusters, err := client.ListVClusters(ctx, cfg.Platform.PlatformNamespace)
	if err != nil {
		fmt.Printf("  %s %s\n", tui.WarningStyle.Render("⚠"), err.Error())
	} else if len(vclusters) == 0 {
		fmt.Println(tui.DimStyle.Render("  (no vclusters)"))
	} else {
		var rows [][]string
		for _, vc := range vclusters {
			name := vc.GetName()
			preset, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "vcluster", "preset")
			hostname, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "exposure", "hostname")
			rows = append(rows, []string{name, preset, hostname})
		}
		fmt.Println(tui.Table([]string{"NAME", "PRESET", "HOSTNAME"}, rows))
	}

	return nil
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("%s %s\n\n", tui.TitleStyle.Render("Diagnosing"), name)

	result, err := platform.DiagnoseVCluster(ctx, client, cfg.Platform.PlatformNamespace, name)
	if err != nil {
		return err
	}

	fmt.Printf("  %s\n", name)
	for i, step := range result.Steps {
		isLast := i == len(result.Steps)-1
		fmt.Println(tui.TreeNode(
			fmt.Sprintf("%-15s", step.Name),
			step.Status.String(),
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

	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	timestamp := git.ReconcileAnnotation()
	err = client.SetReconcileAnnotation(ctx, kube.VClusterOrchestratorV2GVR, cfg.Platform.PlatformNamespace, name, timestamp)
	if err != nil {
		return fmt.Errorf("setting reconcile annotation: %w", err)
	}

	fmt.Printf("%s Set reconcile-at=%s on %s\n", tui.SuccessStyle.Render("✓"), timestamp, name)
	return nil
}

func runContext(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	fmt.Println(tui.TitleStyle.Render("Platform Context"))
	fmt.Println()
	fmt.Printf("  Repo:       %s\n", valueOrDim(cfg.RepoPath))
	fmt.Printf("  Git mode:   %s\n", cfg.GitMode)
	fmt.Printf("  ArgoCD:     %s\n", cfg.ArgocdURL)
	fmt.Printf("  Domain:     %s\n", cfg.Platform.Domain)
	fmt.Printf("  Namespace:  %s\n", cfg.Platform.PlatformNamespace)
	fmt.Printf("  Config:     %s\n", tui.DimStyle.Render(config.ConfigPath()))
	fmt.Println()

	return nil
}

func valueOrDim(v string) string {
	if v == "" {
		return tui.DimStyle.Render("(not set)")
	}
	return v
}
