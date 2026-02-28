package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// --- hctl up ---

var upCmd = &cobra.Command{
	Use:   "up [workload]",
	Short: "Scale a workload to desired replicas",
	Long: `Scales a workload's deployments to the specified replica count (default: 1).
Re-enables ArgoCD auto-sync if it was disabled.

If no workload name is given, reads from score.yaml in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUp,
}

var upReplicas int32

func init() {
	upCmd.Flags().Int32VarP(&upReplicas, "replicas", "r", 1, "target replica count")
}

func runUp(cmd *cobra.Command, args []string) error {
	workloadName, cluster, err := resolveWorkloadAndCluster(args)
	if err != nil {
		return err
	}

	cfg := config.Get()
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespace := cluster // workloads deploy to namespace matching cluster name

	fmt.Printf("\n  %s Scaling up %s in %s (replicas=%d)\n\n",
		tui.InfoStyle.Render(tui.IconArrow), workloadName, cluster, upReplicas)

	deploys, err := client.ListDeployments(ctx, namespace)
	if err != nil {
		return err
	}

	// Filter deployments belonging to this workload
	matched := filterWorkloadDeployments(deploys, workloadName)
	if len(matched) == 0 {
		// Fallback: try all deployments in the namespace
		if len(deploys) > 0 {
			matched = deploys
		} else {
			return fmt.Errorf("no deployments found for %s in namespace %s", workloadName, namespace)
		}
	}

	for _, d := range matched {
		if d.ArgoApp != "" {
			fmt.Printf("    %s Re-enabling auto-sync for %s\n",
				tui.MutedStyle.Render(tui.IconArrow), d.ArgoApp)
			_ = client.EnableArgoAutoSync(ctx, "argocd", d.ArgoApp)
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

var downCmd = &cobra.Command{
	Use:   "down [workload]",
	Short: "Scale a workload to zero",
	Long: `Disables ArgoCD auto-sync and scales a workload's deployments to 0.

If no workload name is given, reads from score.yaml in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDown,
}

func runDown(cmd *cobra.Command, args []string) error {
	workloadName, cluster, err := resolveWorkloadAndCluster(args)
	if err != nil {
		return err
	}

	cfg := config.Get()
	if cfg.Interactive {
		ok, _ := tui.Confirm(fmt.Sprintf("Scale down %s in %s?", workloadName, cluster))
		if !ok {
			fmt.Println(tui.DimStyle.Render("Cancelled"))
			return nil
		}
	}

	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespace := cluster

	fmt.Printf("\n  %s Scaling down %s in %s\n\n",
		tui.WarningStyle.Render(tui.IconArrow), workloadName, cluster)

	deploys, err := client.ListDeployments(ctx, namespace)
	if err != nil {
		return err
	}

	matched := filterWorkloadDeployments(deploys, workloadName)
	if len(matched) == 0 {
		if len(deploys) > 0 {
			matched = deploys
		} else {
			return fmt.Errorf("no deployments found for %s in namespace %s", workloadName, namespace)
		}
	}

	for _, d := range matched {
		if d.ArgoApp != "" {
			fmt.Printf("    %s Disabling auto-sync for %s\n",
				tui.MutedStyle.Render(tui.IconArrow), d.ArgoApp)
			_ = client.DisableArgoAutoSync(ctx, "argocd", d.ArgoApp)
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

var openCmd = &cobra.Command{
	Use:   "open [workload]",
	Short: "Open a workload's URL in the browser",
	Long: `Looks up the workload's HTTPRoute or route resource to find its URL
and opens it in the default browser.

If no workload name is given, reads from score.yaml in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	workloadName, cluster, err := resolveWorkloadAndCluster(args)
	if err != nil {
		return err
	}

	cfg := config.Get()

	// Try to find URL from the route resource in score.yaml
	url := ""
	if w, loadErr := score.LoadWorkload("score.yaml"); loadErr == nil {
		routes := w.ResourcesByType("route")
		for _, r := range routes {
			if host, ok := r.Params["host"].(string); ok {
				url = "https://" + host
				break
			}
		}
	}

	// Fallback: check rendered values for httpRoute host
	if url == "" {
		valuesPath := fmt.Sprintf("workloads/%s/addons/%s/values.yaml", cluster, workloadName)
		absPath := fmt.Sprintf("%s/%s", cfg.RepoPath, valuesPath)
		if data, readErr := os.ReadFile(absPath); readErr == nil {
			content := string(data)
			// Simple heuristic: find host in httpRoute section
			for _, line := range strings.Split(content, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "host:") {
					host := strings.TrimSpace(strings.TrimPrefix(trimmed, "host:"))
					host = strings.Trim(host, "\"'")
					if host != "" {
						url = "https://" + host
						break
					}
				}
			}
		}
	}

	// Fallback: check ArgoCD app for URLs
	if url == "" {
		client, cErr := kube.NewClient(cfg.KubeContext)
		if cErr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			app, aErr := client.GetArgoApp(ctx, "argocd", workloadName)
			if aErr != nil {
				app, aErr = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
			}
			if aErr == nil {
				// Check for URL in annotations
				if meta, ok := app.Object["metadata"].(map[string]interface{}); ok {
					if anns, ok := meta["annotations"].(map[string]interface{}); ok {
						if u, ok := anns["hctl.integratn.tech/url"].(string); ok {
							url = u
						}
					}
				}
			}
		}
	}

	if url == "" {
		return fmt.Errorf("could not determine URL for %s — add a route resource to score.yaml or set hctl.integratn.tech/url annotation", workloadName)
	}

	fmt.Printf("  %s Opening %s\n", tui.InfoStyle.Render(tui.IconArrow), url)
	return openBrowser(url)
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported OS for browser open: %s", runtime.GOOS)
	}
	return exec.Command(cmd, args...).Start()
}

// --- hctl logs ---

var logsCmd = &cobra.Command{
	Use:   "logs [workload]",
	Short: "Stream logs for a workload's pods",
	Long: `Finds pods belonging to a workload and streams their logs.

If no workload name is given, reads from score.yaml in the current directory.
Use --follow to stream continuously (like kubectl logs -f).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
}

var (
	logsFollow    bool
	logsTail      int64
	logsContainer string
)

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream logs continuously")
	logsCmd.Flags().Int64VarP(&logsTail, "tail", "t", 100, "number of recent lines to show")
	logsCmd.Flags().StringVarP(&logsContainer, "container", "c", "", "specific container name")
}

func runLogs(cmd *cobra.Command, args []string) error {
	workloadName, cluster, err := resolveWorkloadAndCluster(args)
	if err != nil {
		return err
	}

	cfg := config.Get()
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	namespace := cluster

	ctx := context.Background()
	if !logsFollow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Find pods for this workload
	pods, err := client.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}
	if len(pods) == 0 {
		// Try broader label
		pods, err = client.ListPods(ctx, namespace, fmt.Sprintf("app=%s", workloadName))
		if err != nil || len(pods) == 0 {
			return fmt.Errorf("no pods found for workload %s in namespace %s", workloadName, namespace)
		}
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

// --- Shared helpers ---

// resolveWorkloadAndCluster determines the workload name and target cluster from
// command arguments or by reading score.yaml in the current directory.
func resolveWorkloadAndCluster(args []string) (workloadName, cluster string, err error) {
	cfg := config.Get()

	if len(args) > 0 {
		workloadName = args[0]
	}

	// Try to load score.yaml for metadata
	if w, loadErr := score.LoadWorkload("score.yaml"); loadErr == nil {
		if workloadName == "" {
			workloadName = w.Metadata.Name
		}
		if cluster == "" {
			cluster = w.TargetCluster()
		}
	}

	if workloadName == "" {
		return "", "", fmt.Errorf("no workload specified and no score.yaml found in current directory")
	}

	if cluster == "" {
		cluster = cfg.DefaultCluster
	}
	if cluster == "" {
		return "", "", fmt.Errorf("no cluster specified — use config defaultCluster or have score.yaml with cluster annotation")
	}

	return workloadName, cluster, nil
}

// filterWorkloadDeployments returns deployments whose name contains the workload name
// or whose ArgoCD app matches.
func filterWorkloadDeployments(deploys []kube.DeploymentInfo, workloadName string) []kube.DeploymentInfo {
	var matched []kube.DeploymentInfo
	for _, d := range deploys {
		if d.Name == workloadName || strings.Contains(d.Name, workloadName) || d.ArgoApp == workloadName {
			matched = append(matched, d)
		}
	}
	return matched
}
