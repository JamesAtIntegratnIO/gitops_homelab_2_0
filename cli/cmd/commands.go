package cmd

import (
	"context"
	"fmt"
	"os"
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
		return err
	}

	// Check if any steps failed
	for _, r := range results {
		if r.Err != nil {
			return fmt.Errorf("init failed at %q: %w", r.Title, r.Err)
		}
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	// Structured output mode: collect all status as JSON/YAML
	if tui.IsStructured() {
		if watchFlag {
			return runStatusWatch(cfg)
		}
		return runStatusOnce(cfg)
	}

	return tui.RunDashboard(tui.IconPlay+" Platform Status", []tui.DashboardSection{
		{
			Title: "Nodes",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				nodes, err := client.ListNodes(ctx)
				if err != nil {
					return "", err
				}
				var rows [][]string
				for _, n := range nodes {
					status := tui.StatusIcon(n.Ready)
					rows = append(rows, []string{n.Name, status, n.IP, strings.Join(n.Roles, ","), n.CPU, n.Memory})
				}
				return tui.Table([]string{"NAME", "READY", "IP", "ROLES", "CPU", "MEMORY"}, rows), nil
			},
		},
		{
			Title: "ArgoCD",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				apps, err := client.ListArgoApps(ctx, "argocd")
				if err != nil {
					return "", err
				}
				synced, outOfSync, degraded, healthy := 0, 0, 0, 0
				var unhealthyRows [][]string
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
					if syncStatus != "Synced" || healthStatus != "Healthy" {
						unhealthyRows = append(unhealthyRows, []string{
							app.GetName(),
							syncStatus,
							healthStatus,
						})
					}
				}

				var sb strings.Builder
				summary := fmt.Sprintf("  Total: %d  │  Synced: %s  │  OutOfSync: %s  │  Healthy: %s  │  Degraded: %s\n",
					len(apps),
					tui.SuccessStyle.Render(fmt.Sprintf("%d", synced)),
					tui.WarningStyle.Render(fmt.Sprintf("%d", outOfSync)),
					tui.SuccessStyle.Render(fmt.Sprintf("%d", healthy)),
					tui.ErrorStyle.Render(fmt.Sprintf("%d", degraded)),
				)
				sb.WriteString(summary)

				if len(unhealthyRows) > 0 {
					sb.WriteString("\n" + tui.WarningStyle.Render("  Unhealthy Applications:") + "\n")
					sb.WriteString(tui.Table([]string{"NAME", "SYNC", "HEALTH"}, unhealthyRows))
				}
				return sb.String(), nil
			},
		},
		{
			Title: "Promises",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				promises, err := client.ListPromises(ctx)
				if err != nil {
					return "", err
				}
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
				return tui.Table([]string{"PROMISE", "STATUS"}, rows), nil
			},
		},
		{
			Title: "vClusters",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				vclusters, err := client.ListVClusters(ctx, cfg.Platform.PlatformNamespace)
				if err != nil {
					return "", err
				}
				if len(vclusters) == 0 {
					return tui.DimStyle.Render("  (no vclusters)"), nil
				}
				var rows [][]string
				for _, vc := range vclusters {
					name := vc.GetName()
					preset, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "vcluster", "preset")
					hostname, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "exposure", "hostname")

					argoApp, err := client.GetArgoApp(ctx, "argocd", name)
					health := tui.DimStyle.Render("unknown")
					if err == nil {
						syncStatus, _, _ := platform.UnstructuredNestedString(argoApp.Object, "status", "sync", "status")
						healthStatus, _, _ := platform.UnstructuredNestedString(argoApp.Object, "status", "health", "status")
						if syncStatus == "Synced" && healthStatus == "Healthy" {
							health = tui.SuccessStyle.Render("Healthy")
						} else {
							health = tui.WarningStyle.Render(syncStatus + "/" + healthStatus)
						}
					}
					rows = append(rows, []string{name, preset, hostname, health})
				}
				return tui.Table([]string{"NAME", "PRESET", "HOSTNAME", "STATUS"}, rows), nil
			},
		},
		{
			Title: "Workloads",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Get all addon=true apps and group by vcluster
				ps, err := platform.CollectPlatformStatus(ctx, client, cfg.Platform.PlatformNamespace)
				if err != nil {
					return "", err
				}
				if len(ps.Workloads) == 0 {
					return tui.DimStyle.Render("  (no workloads deployed)"), nil
				}

				var rows [][]string
				for _, w := range ps.Workloads {
					cluster := w.Labels["clusterName"]
					phase := phaseStyled(w.Phase)
					rows = append(rows, []string{w.Name, cluster, w.Namespace, w.ArgoCD.SyncStatus, phase})
				}
				return tui.Table([]string{"NAME", "CLUSTER", "NAMESPACE", "SYNC", "STATUS"}, rows), nil
			},
		},
		{
			Title: "Addons",
			Load: func() (string, error) {
				client, err := kube.NewClient(cfg.KubeContext)
				if err != nil {
					return "", err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				ps, err := platform.CollectPlatformStatus(ctx, client, cfg.Platform.PlatformNamespace)
				if err != nil {
					return "", err
				}
				if len(ps.Addons) == 0 {
					return tui.DimStyle.Render("  (no addons)"), nil
				}

				// Group by environment
				envGroups := make(map[string][]platform.ResourceStatus)
				for _, a := range ps.Addons {
					env := a.Labels["environment"]
					if env == "" {
						env = "(unset)"
					}
					envGroups[env] = append(envGroups[env], a)
				}

				var sb strings.Builder
				for env, addons := range envGroups {
					sb.WriteString(fmt.Sprintf("\n  %s\n", tui.TitleStyle.Render(env)))
					var rows [][]string
					for _, a := range addons {
						phase := phaseStyled(a.Phase)
						rows = append(rows, []string{a.Name, a.Namespace, a.ArgoCD.SyncStatus, phase})
					}
					sb.WriteString(tui.Table([]string{"NAME", "NAMESPACE", "SYNC", "STATUS"}, rows))
				}
				return sb.String(), nil
			},
		},
	})
}

// phaseStyled returns a styled phase string for TUI display.
func phaseStyled(phase string) string {
	switch phase {
	case "Ready":
		return tui.SuccessStyle.Render("Ready")
	case "Progressing":
		return tui.InfoStyle.Render("Progressing")
	case "Degraded":
		return tui.WarningStyle.Render("Degraded")
	case "Failed":
		return tui.ErrorStyle.Render("Failed")
	case "Suspended":
		return tui.DimStyle.Render("Suspended")
	default:
		return tui.DimStyle.Render(phase)
	}
}

// runStatusOnce collects platform status and prints it once in structured format.
func runStatusOnce(cfg *config.Config) error {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps, err := platform.CollectPlatformStatus(ctx, client, cfg.Platform.PlatformNamespace)
	if err != nil {
		return err
	}
	return tui.RenderOutput(ps, "")
}

// runStatusWatch continuously polls and prints platform status in structured format.
func runStatusWatch(cfg *config.Config) error {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	// Print immediately, then on each tick
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		ps, err := platform.CollectPlatformStatus(ctx, client, cfg.Platform.PlatformNamespace)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			_ = tui.RenderOutput(ps, "")
		}

		select {
		case <-ticker.C:
			continue
		}
	}
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	var result *platform.DiagnosticResult

	_, err := tui.Spin("Diagnosing "+name, func() (string, error) {
		client, err := kube.NewClient(cfg.KubeContext)
		if err != nil {
			return "", fmt.Errorf("connecting to cluster: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err = platform.DiagnoseVCluster(ctx, client, cfg.Platform.PlatformNamespace, name)
		return "", err
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n  %s\n", name)
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

	err = client.SetManualReconciliationLabel(ctx, kube.VClusterOrchestratorV2GVR, cfg.Platform.PlatformNamespace, name)
	if err != nil {
		return fmt.Errorf("setting reconciliation label: %w", err)
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
