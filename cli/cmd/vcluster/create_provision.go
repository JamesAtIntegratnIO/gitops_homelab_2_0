package vcluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"gopkg.in/yaml.v3"
)

// writeAndCommitVCluster marshals the VClusterSpec, writes it to disk, and runs the git workflow.
// Returns true if the resource was committed.
func writeAndCommitVCluster(cfg *config.Config, opts *CreateOptions, name, preset string, spec platform.VClusterSpec, interactive bool) (bool, error) {
	resource := platform.NewVClusterResource(spec, cfg.Platform.PlatformNamespace)

	data, err := yaml.Marshal(resource)
	if err != nil {
		return false, fmt.Errorf("marshaling resource: %w", err)
	}

	// Show preview
	fmt.Println(tui.TitleStyle.Render("Generated VClusterOrchestratorV2"))
	fmt.Println(tui.DimStyle.Render("---"))
	fmt.Println(string(data))

	// Write file
	repoPath := cfg.RepoPath
	if repoPath == "" {
		repo, err := git.DetectRepo("")
		if err != nil {
			return false, hcerrors.NewUserError("cannot detect repo — run 'hctl init' first or set repoPath in config")
		}
		repoPath = repo.Root
	}

	outPath := filepath.Join(repoPath, "platform", "vclusters", name+".yaml")
	if _, err := os.Stat(outPath); err == nil {
		if interactive {
			confirmed, confirmErr := tui.Confirm(fmt.Sprintf("File %s already exists. Overwrite?", outPath))
			if confirmErr != nil {
				return false, hcerrors.NewUserError("confirming operation: %w", confirmErr)
			}
			if !confirmed {
				return false, hcerrors.NewUserError("cancelled")
			}
		} else {
			return false, hcerrors.NewUserError("file already exists: %s (use --auto-commit with caution)", outPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return false, fmt.Errorf("creating directory: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return false, fmt.Errorf("writing file: %w", err)
	}

	relPath, _ := filepath.Rel(repoPath, outPath)
	fmt.Printf("\n%s Written to %s\n", tui.SuccessStyle.Render(tui.IconCheck), relPath)

	// Git handling
	gitMode := cfg.GitMode
	if opts.AutoCommit {
		gitMode = "auto"
	}

	gitResult, err := git.HandleGitWorkflow(git.WorkflowOpts{
		RepoPath:    repoPath,
		Paths:       []string{relPath},
		Action:      "create vcluster",
		Resource:    name,
		Details:     fmt.Sprintf("%s, %d replicas", preset, spec.VCluster.Replicas),
		GitMode:     gitMode,
		Interactive: interactive,
		UI:          tui.GitUIAdapter{},
	})
	if err != nil {
		return false, fmt.Errorf("running git workflow: %w", err)
	}

	fmt.Printf("\n%s\n", tui.DimStyle.Render("Next: ArgoCD will sync the resource and Kratix will provision the vCluster."))
	fmt.Printf("%s\n", tui.DimStyle.Render("Monitor with: hctl vcluster status "+name))

	return gitResult == git.GitCommitted, nil
}

// watchProvisioning runs the animated provisioning wait sequence.
func watchProvisioning(cfg *config.Config, opts *CreateOptions, name, hostname string, spec platform.VClusterSpec) error {
	client, err := kube.Shared()
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ns := cfg.Platform.PlatformNamespace
	timeout := time.Duration(opts.Timeout) * time.Second
	poll := 3 * time.Second

	steps := []tui.Step{
		{
			Title: "Request accepted",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForRequest(ctx, client, ns, name, poll)
			},
		},
		{
			Title: "Pipeline running",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForPipeline(ctx, client, ns, name, poll)
			},
		},
		{
			Title: "ArgoCD syncing",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForArgoSync(ctx, client, name, poll)
			},
		},
		{
			Title: "Cluster ready",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForClusterReady(ctx, client, name, poll)
			},
		},
	}

	fmt.Println()
	_, err = tui.RunSteps("Provisioning "+name, steps)
	if err != nil {
		return fmt.Errorf("provisioning %s: %w", name, err)
	}

	// Collect and display summary
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := platform.CollectProvisionResult(ctx, client, ns, name)
	if err != nil {
		// Non-fatal — provisioning succeeded but summary collection failed
		fmt.Printf("\n  %s %s is ready!\n", tui.SuccessStyle.Render(tui.IconCheck), name)
		fmt.Printf("  %s\n", tui.DimStyle.Render("Run 'hctl vcluster status "+name+"' for details"))
		return nil
	}

	fmt.Print(FormatProvisionSummary(result, hostname))
	return nil
}
