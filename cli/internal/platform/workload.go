package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/score"
)

// ResolveWorkloadAndCluster determines the workload name and target cluster from
// command arguments or by reading score.yaml in the current directory.
// defaultCluster is used as a fallback when no cluster can be determined from
// score.yaml metadata.
func ResolveWorkloadAndCluster(args []string, defaultCluster string) (workloadName, cluster string, err error) {
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
		cluster = defaultCluster
	}
	if cluster == "" {
		return "", "", fmt.Errorf("no cluster specified — use config defaultCluster or have score.yaml with cluster annotation")
	}

	return workloadName, cluster, nil
}

// FilterWorkloadDeployments returns deployments whose name contains the workload
// name or whose ArgoCD app matches.
func FilterWorkloadDeployments(deploys []kube.DeploymentInfo, workloadName string) []kube.DeploymentInfo {
	var matched []kube.DeploymentInfo
	for _, d := range deploys {
		if d.Name == workloadName || strings.Contains(d.Name, workloadName) || d.ArgoApp == workloadName {
			matched = append(matched, d)
		}
	}
	return matched
}

// MatchDeployments filters deployments for a workload, falling back to all
// deployments in the namespace if no specific matches are found. Returns an
// error only when both the filter and fallback produce no results.
func MatchDeployments(deploys []kube.DeploymentInfo, workloadName, namespace string) ([]kube.DeploymentInfo, error) {
	matched := FilterWorkloadDeployments(deploys, workloadName)
	if len(matched) == 0 {
		if len(deploys) > 0 {
			return deploys, nil
		}
		return nil, fmt.Errorf("no deployments found for %s in namespace %s", workloadName, namespace)
	}
	return matched, nil
}

// ResolveWorkloadURL attempts to determine the URL for a workload by checking
// (in order): score.yaml route resources, rendered values files, and ArgoCD
// application annotations. Returns an empty string and error if no URL can be
// determined.
func ResolveWorkloadURL(workloadName, cluster, repoPath, kubeContext string) (string, error) {
	// 1. Try score.yaml route resources
	if w, loadErr := score.LoadWorkload("score.yaml"); loadErr == nil {
		routes := w.ResourcesByType("route")
		for _, r := range routes {
			if host, ok := r.Params["host"].(string); ok {
				return "https://" + host, nil
			}
		}
	}

	// 2. Fallback: check rendered values for httpRoute host
	valuesPath := fmt.Sprintf("workloads/%s/addons/%s/values.yaml", cluster, workloadName)
	absPath := fmt.Sprintf("%s/%s", repoPath, valuesPath)
	if data, readErr := os.ReadFile(absPath); readErr == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "host:") {
				host := strings.TrimSpace(strings.TrimPrefix(trimmed, "host:"))
				host = strings.Trim(host, "\"'")
				if host != "" {
					return "https://" + host, nil
				}
			}
		}
	}

	// 3. Fallback: check ArgoCD app for URL annotation
	if kubeContext != "" {
		client, cErr := kube.NewClient(kubeContext)
		if cErr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			app, aErr := client.GetArgoApp(ctx, "argocd", workloadName)
			if aErr != nil {
				app, aErr = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
			}
			if aErr == nil {
				if meta, ok := app.Object["metadata"].(map[string]interface{}); ok {
					if anns, ok := meta["annotations"].(map[string]interface{}); ok {
						if u, ok := anns["hctl.integratn.tech/url"].(string); ok {
							return u, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine URL for %s — add a route resource to score.yaml or set hctl.integratn.tech/url annotation", workloadName)
}

// FindWorkloadPods locates pods for a workload in the given namespace, trying
// the standard app.kubernetes.io/name label first, then falling back to the
// simpler app= label.
func FindWorkloadPods(ctx context.Context, client KubeClient, namespace, workloadName string) ([]kube.PodInfo, error) {
	pods, err := client.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	if len(pods) == 0 {
		// Try broader label
		pods, err = client.ListPods(ctx, namespace, fmt.Sprintf("app=%s", workloadName))
		if err != nil {
			return nil, fmt.Errorf("listing pods: %w", err)
		}
		if len(pods) == 0 {
			return nil, fmt.Errorf("no pods found for workload %s in namespace %s", workloadName, namespace)
		}
	}
	return pods, nil
}

// OpenBrowser opens the given URL in the default system browser.
func OpenBrowser(url string) error {
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
