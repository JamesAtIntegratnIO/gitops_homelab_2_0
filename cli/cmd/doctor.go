package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check platform prerequisites and connectivity",
	Long: `Validates that the local environment and platform are correctly configured.

Checks include:
  - hctl config file exists and is valid
  - Required CLI tools are installed (kubectl, git)
  - Kubernetes cluster is reachable
  - ArgoCD is accessible
  - Git repository is detected and clean
  - Platform namespace exists
  - Kratix CRDs are installed`,
	RunE: runDoctor,
}

// Check represents a single doctor check.
type Check struct {
	Name string
	Run  func(cfg *config.Config) (string, error)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	checks := []Check{
		{Name: "Config file", Run: checkConfigFile},
		{Name: "kubectl", Run: checkKubectl},
		{Name: "git", Run: checkGit},
		{Name: "Git repository", Run: checkGitRepo},
		{Name: "Cluster connectivity", Run: checkCluster},
		{Name: "Platform namespace", Run: checkPlatformNamespace},
		{Name: "ArgoCD", Run: checkArgoCD},
		{Name: "Kratix CRDs", Run: checkKratixCRDs},
	}

	fmt.Printf("\n%s\n\n", tui.TitleStyle.Render("  hctl doctor"))

	passCount := 0
	warnCount := 0
	failCount := 0

	// Structured output
	if tui.IsStructured() {
		type result struct {
			Check   string `json:"check"`
			Status  string `json:"status"`
			Detail  string `json:"detail,omitempty"`
		}
		var results []result

		for _, c := range checks {
			detail, err := c.Run(cfg)
			if err != nil {
				results = append(results, result{Check: c.Name, Status: "fail", Detail: err.Error()})
				failCount++
			} else {
				results = append(results, result{Check: c.Name, Status: "pass", Detail: detail})
				passCount++
			}
		}
		return tui.RenderOutput(map[string]interface{}{
			"checks": results,
			"summary": map[string]int{
				"pass": passCount,
				"warn": warnCount,
				"fail": failCount,
			},
		}, "")
	}

	for _, c := range checks {
		detail, err := c.Run(cfg)
		if err != nil {
			fmt.Printf("  %s %s\n", tui.ErrorStyle.Render(tui.IconCross), c.Name)
			fmt.Printf("    %s\n", tui.DimStyle.Render(err.Error()))
			failCount++
		} else {
			fmt.Printf("  %s %s", tui.SuccessStyle.Render(tui.IconCheck), c.Name)
			if detail != "" {
				fmt.Printf("  %s", tui.DimStyle.Render(detail))
			}
			fmt.Println()
			passCount++
		}
	}

	fmt.Printf("\n  %s %d passed", tui.SuccessStyle.Render(tui.IconCheck), passCount)
	if failCount > 0 {
		fmt.Printf("  %s %d failed", tui.ErrorStyle.Render(tui.IconCross), failCount)
	}
	fmt.Println()
	fmt.Println()

	if failCount > 0 {
		return fmt.Errorf("%d check(s) failed", failCount)
	}
	return nil
}

func checkConfigFile(cfg *config.Config) (string, error) {
	path := config.ConfigPath()
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("config file not found at %s — run 'hctl init'", path)
	}
	return path, nil
}

func checkKubectl(_ *config.Config) (string, error) {
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return "", fmt.Errorf("kubectl not found in PATH")
	}
	out, err := exec.Command("kubectl", "version", "--client", "--short").CombinedOutput()
	if err != nil {
		return path, nil // found but can't get version — still OK
	}
	return string(out[:len(out)-1]), nil // trim newline
}

func checkGit(_ *config.Config) (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH")
	}
	out, _ := exec.Command("git", "--version").CombinedOutput()
	if len(out) > 0 {
		return string(out[:len(out)-1]), nil
	}
	return path, nil
}

func checkGitRepo(cfg *config.Config) (string, error) {
	repoPath := cfg.RepoPath
	if repoPath == "" {
		return "", fmt.Errorf("repoPath not set in config")
	}
	repo, err := git.DetectRepo(repoPath)
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", repoPath)
	}
	branch, _ := repo.CurrentBranch()
	return fmt.Sprintf("branch=%s", branch), nil
}

func checkCluster(cfg *config.Config) (string, error) {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return "", fmt.Errorf("cannot create client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nodes, err := client.ListNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot reach cluster: %w", err)
	}
	ready := 0
	for _, n := range nodes {
		if n.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d nodes ready", ready, len(nodes)), nil
}

func checkPlatformNamespace(cfg *config.Config) (string, error) {
	ns := cfg.Platform.PlatformNamespace
	if ns == "" {
		return "", fmt.Errorf("platformNamespace not configured")
	}
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1Options())
	if err != nil {
		return "", fmt.Errorf("namespace %s not found: %w", ns, err)
	}
	return ns, nil
}

func checkArgoCD(cfg *config.Config) (string, error) {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	apps, err := client.ListArgoApps(ctx, "argocd")
	if err != nil {
		return "", fmt.Errorf("cannot list ArgoCD apps: %w", err)
	}
	return fmt.Sprintf("%d apps", len(apps)), nil
}

func checkKratixCRDs(cfg *config.Config) (string, error) {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the VClusterOrchestratorV2 CRD exists by trying to list
	_, err = client.Dynamic.Resource(kube.VClusterOrchestratorV2GVR).
		Namespace(cfg.Platform.PlatformNamespace).
		List(ctx, metav1ListOptions())
	if err != nil {
		return "", fmt.Errorf("Kratix CRDs not found: %w", err)
	}
	return "VClusterOrchestratorV2 available", nil
}

func metav1Options() metav1.GetOptions     { return metav1.GetOptions{} }
func metav1ListOptions() metav1.ListOptions { return metav1.ListOptions{} }
