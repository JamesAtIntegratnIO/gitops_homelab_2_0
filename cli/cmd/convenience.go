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

// --- hctl up ---

var upReplicas int32

func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [workload]",
		Short: "Scale a workload to desired replicas",
		Long: `Scales a workload's deployments to the specified replica count (default: 1).
Re-enables ArgoCD auto-sync if it was disabled.

If no workload name is given, reads from score.yaml in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runUp,
	}
	cmd.Flags().Int32VarP(&upReplicas, "replicas", "r", 1, "target replica count")
	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	workloadName, cluster, err := platform.ResolveWorkloadAndCluster(args, cfg.DefaultCluster)
	if err != nil {
		return hcerrors.NewUserError("resolving workload: %w", err)
	}

	client, err := kube.Shared()
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespace := cluster // workloads deploy to namespace matching cluster name

	fmt.Printf("\n  %s Scaling up %s in %s (replicas=%d)\n\n",
		tui.InfoStyle.Render(tui.IconArrow), workloadName, cluster, upReplicas)

	deploys, err := client.ListDeployments(ctx, namespace)
	if err != nil {
		return hcerrors.NewPlatformError("listing deployments: %w", err)
	}

	matched, err := platform.MatchDeployments(deploys, workloadName, namespace)
	if err != nil {
		return hcerrors.NewUserError("matching deployments: %w", err)
	}

	for _, d := range matched {
		if d.ArgoApp != "" {
			fmt.Printf("    %s Re-enabling auto-sync for %s\n",
				tui.MutedStyle.Render(tui.IconArrow), d.ArgoApp)
			if err := client.EnableArgoAutoSync(ctx, "argocd", d.ArgoApp); err != nil {
				fmt.Printf("    %s Failed to enable auto-sync for %s: %v\n", tui.WarningStyle.Render(tui.IconWarn), d.ArgoApp, err)
			}
		}

		fmt.Printf("    %s Scaling %s to %d\n",
			tui.MutedStyle.Render(tui.IconArrow), d.Name, upReplicas)
		if err := client.ScaleDeployment(ctx, namespace, d.Name, upReplicas); err != nil {
			fmt.Printf("    %s Failed: %v\n", tui.WarningStyle.Render(tui.IconWarn), err)
		}
	}

	fmt.Printf("\n  %s %s scaled up in %s\n", tui.SuccessStyle.Render(tui.IconCheck), workloadName, cluster)
	return nil
}

// --- hctl down ---

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [workload]",
		Short: "Scale a workload to zero",
		Long: `Disables ArgoCD auto-sync and scales a workload's deployments to 0.

If no workload name is given, reads from score.yaml in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runDown,
	}
}

func runDown(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	workloadName, cluster, err := platform.ResolveWorkloadAndCluster(args, cfg.DefaultCluster)
	if err != nil {
		return hcerrors.NewUserError("resolving workload: %w", err)
	}

	if cfg.Interactive {
		ok, _ := tui.Confirm(fmt.Sprintf("Scale down %s in %s?", workloadName, cluster))
		if !ok {
			fmt.Println(tui.DimStyle.Render("Cancelled"))
			return nil
		}
	}

	client, err := kube.Shared()
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespace := cluster

	fmt.Printf("\n  %s Scaling down %s in %s\n\n",
		tui.WarningStyle.Render(tui.IconArrow), workloadName, cluster)

	deploys, err := client.ListDeployments(ctx, namespace)
	if err != nil {
		return hcerrors.NewPlatformError("listing deployments: %w", err)
	}

	matched, err := platform.MatchDeployments(deploys, workloadName, namespace)
	if err != nil {
		return hcerrors.NewUserError("matching deployments: %w", err)
	}

	for _, d := range matched {
		if d.ArgoApp != "" {
			fmt.Printf("    %s Disabling auto-sync for %s\n",
				tui.MutedStyle.Render(tui.IconArrow), d.ArgoApp)
			if err := client.DisableArgoAutoSync(ctx, "argocd", d.ArgoApp); err != nil {
				fmt.Printf("    %s Failed to disable auto-sync for %s: %v\n", tui.WarningStyle.Render(tui.IconWarn), d.ArgoApp, err)
			}
		}

		fmt.Printf("    %s Scaling %s to 0\n",
			tui.MutedStyle.Render(tui.IconArrow), d.Name)
		if err := client.ScaleDeployment(ctx, namespace, d.Name, 0); err != nil {
			fmt.Printf("    %s Failed: %v\n", tui.WarningStyle.Render(tui.IconWarn), err)
		}
	}

	fmt.Printf("\n  %s %s scaled down in %s\n", tui.SuccessStyle.Render(tui.IconCheck), workloadName, cluster)
	return nil
}

// --- hctl open ---

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [workload]",
		Short: "Open a workload's URL in the browser",
		Long: `Looks up the workload's HTTPRoute or route resource to find its URL
and opens it in the default browser.

If no workload name is given, reads from score.yaml in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runOpen,
	}
}

func runOpen(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	workloadName, cluster, err := platform.ResolveWorkloadAndCluster(args, cfg.DefaultCluster)
	if err != nil {
		return hcerrors.NewUserError("resolving workload: %w", err)
	}

	url, err := platform.ResolveWorkloadURL(workloadName, cluster, cfg.RepoPath, cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("resolving workload URL: %w", err)
	}

	fmt.Printf("  %s Opening %s\n", tui.InfoStyle.Render(tui.IconArrow), url)
	return platform.OpenBrowser(url)
}

// --- hctl logs ---

var (
	logsFollow    bool
	logsTail      int64
	logsContainer string
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [workload]",
		Short: "Stream logs for a workload's pods",
		Long: `Finds pods belonging to a workload and streams their logs.

If no workload name is given, reads from score.yaml in the current directory.
Use --follow to stream continuously (like kubectl logs -f).`,
		Args: cobra.MaximumNArgs(1),
		RunE: runLogs,
	}
	cmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream logs continuously")
	cmd.Flags().Int64VarP(&logsTail, "tail", "t", 100, "number of recent lines to show")
	cmd.Flags().StringVarP(&logsContainer, "container", "c", "", "specific container name")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	workloadName, cluster, err := platform.ResolveWorkloadAndCluster(args, cfg.DefaultCluster)
	if err != nil {
		return hcerrors.NewUserError("resolving workload: %w", err)
	}

	client, err := kube.Shared()
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	namespace := cluster

	ctx := context.Background()
	if !logsFollow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	pods, err := platform.FindWorkloadPods(ctx, client, namespace, workloadName)
	if err != nil {
		return hcerrors.NewPlatformError("finding workload pods: %w", err)
	}

	if len(pods) == 1 {
		pod := pods[0]
		fmt.Fprintf(os.Stderr, "%s Streaming logs from %s\n\n",
			tui.InfoStyle.Render(tui.IconArrow), pod.Name)
		return client.StreamPodLogs(ctx, namespace, pod.Name, logsContainer, logsFollow, logsTail, os.Stdout)
	}

	// Multiple pods — show selection or stream all
	fmt.Fprintf(os.Stderr, "%s Found %d pods, streaming from first: %s\n\n",
		tui.InfoStyle.Render(tui.IconArrow), len(pods), pods[0].Name)
	return client.StreamPodLogs(ctx, namespace, pods[0].Name, logsContainer, logsFollow, logsTail, os.Stdout)
}
