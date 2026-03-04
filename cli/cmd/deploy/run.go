package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDeployRunCmd() *cobra.Command {
	var (
		cluster      string
		dryRun       bool
		scoreFile    string
		watchDeploy  bool
		watchTimeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Deploy a Score workload to the platform",
		Long: `Translates score.yaml into platform resources and deploys to a vCluster.

Generates:
  - Stakater Application Helm chart values.yaml
  - ExternalSecrets for database/service credentials (via 1Password)
  - HTTPRoutes + TLS Certificates for ingress
  - PVCs for persistent volumes (NFS via democratic-csi)

Files are written to workloads/<cluster>/addons/<workload>/ in the gitops repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return hcerrors.NewUserError("repo path not set \u2014 run 'hctl init'")
			}

			// Phase 1: Parse and translate (spinner)
			var workload *score.Workload
			var result *deploylib.TranslateResult

			results, err := tui.RunSteps("Preparing deployment", []tui.Step{
				{
					Title: "Parsing " + scoreFile,
					Run: func() (string, error) {
						w, err := score.LoadWorkload(scoreFile)
						if err != nil {
							return "", fmt.Errorf("loading score workload: %w", err)
						}
						workload = w
						return workload.Metadata.Name, nil
					},
				},
				{
					Title: "Translating to platform resources",
					Run: func() (string, error) {
						r, err := deploylib.Translate(workload, cluster, cfg)
						if err != nil {
							return "", fmt.Errorf("translating workload: %w", err)
						}
						result = r
						resources := []string{}
						for name, res := range workload.Resources {
							resources = append(resources, fmt.Sprintf("%s(%s)", name, res.Type))
						}
						detail := fmt.Sprintf("cluster=%s ns=%s", r.TargetCluster, r.Namespace)
						if len(resources) > 0 {
							detail += " resources=" + strings.Join(resources, ",")
						}
						return detail, nil
					},
				},
			})
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Err != nil {
					return r.Err
				}
			}

			// Show what will be generated
			fmt.Printf("\n  Files to write:\n")
			for path := range result.Files {
				fmt.Printf("    %s %s\n", tui.SuccessStyle.Render(tui.IconBullet), path)
			}
			fmt.Printf("    %s workloads/%s/addons.yaml\n", tui.SuccessStyle.Render(tui.IconBullet), result.TargetCluster)

			// Dry-run mode — show generated values and exit
			if dryRun {
				fmt.Printf("\n%s\n\n", tui.TitleStyle.Render("Generated values.yaml:"))
				out, _ := yaml.Marshal(result.StakaterValues)
				fmt.Println(string(out))
				return nil
			}

			// Confirm
			if cfg.Interactive {
				ok, _ := tui.Confirm("\nDeploy this workload?")
				if !ok {
					fmt.Println(tui.DimStyle.Render("Cancelled"))
					return nil
				}
			}

			// Phase 2: Write and commit (spinner)
			var writtenPaths []string
			deploySteps := []tui.Step{
				{
					Title: "Writing files",
					Run: func() (string, error) {
						wp, err := deploylib.WriteResult(result, cfg.RepoPath)
						if err != nil {
							return "", fmt.Errorf("writing files: %w", err)
						}
						writtenPaths = wp
						return fmt.Sprintf("%d files", len(wp)), nil
					},
				},
			}

			// Add git step based on mode
			gitMode := cfg.GitMode
			if gitMode == "prompt" && cfg.Interactive {
				ok, _ := tui.Confirm("Commit and push changes?")
				if ok {
					gitMode = "auto"
				} else {
					gitMode = "stage-only"
				}
			}
			gitStep := gitWorkflowStep(git.WorkflowOpts{
				RepoPath: cfg.RepoPath,
				Paths:    writtenPaths,
				Action:   "deploy",
				Resource: workload.Metadata.Name,
				Details:  result.TargetCluster,
				GitMode:  gitMode,
			})
			deploySteps = append(deploySteps, gitStep)

			results, err = tui.RunSteps("Deploying "+workload.Metadata.Name, deploySteps)
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Err != nil {
					return hcerrors.NewPlatformError("deploy failed at %q: %w", r.Title, r.Err)
				}
			}

			if !watchDeploy {
				fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will sync the workload automatically."))
				fmt.Printf("%s\n", tui.DimStyle.Render(fmt.Sprintf("Check status: hctl deploy status %s", workload.Metadata.Name)))
				return nil
			}

			// Watch ArgoCD sync progress
			fmt.Printf("\n%s Watching ArgoCD sync...\n\n", tui.InfoStyle.Render(tui.IconSync))
			targetCluster := result.TargetCluster
			client, cErr := kube.Shared()
			if cErr != nil {
				return hcerrors.NewPlatformError("connecting to cluster for watch: %v", cErr)
			}

			deadline := time.Now().Add(watchTimeout)
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			for {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				app, aErr := client.GetArgoApp(ctx, "argocd", workload.Metadata.Name)
				if aErr != nil {
					app, aErr = client.GetArgoApp(ctx, "argocd", targetCluster+"-"+workload.Metadata.Name)
				}
				cancel()

				if aErr == nil {
					syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
					healthStatus := unstr.MustString(app.Object, "status", "health", "status")
					phase := fmt.Sprintf("%s/%s", syncStatus, healthStatus)

					if syncStatus == "Synced" && healthStatus == "Healthy" {
						fmt.Printf("  %s %s\n", tui.SuccessStyle.Render(tui.IconCheck), phase)
						fmt.Printf("\n%s\n", tui.SuccessStyle.Render("Deployment healthy!"))
						return nil
					}
					fmt.Printf("  %s %s\n", tui.WarningStyle.Render(tui.IconDot), phase)
				} else {
					fmt.Printf("  %s waiting for ArgoCD app...\n", tui.DimStyle.Render(tui.IconDot))
				}

				if time.Now().After(deadline) {
					return hcerrors.NewTimeoutError("timeout waiting for sync after %s", watchTimeout)
				}

				<-ticker.C
			}
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show generated resources without writing")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	cmd.Flags().BoolVarP(&watchDeploy, "watch", "w", false, "watch ArgoCD sync after deploy")
	cmd.Flags().DurationVar(&watchTimeout, "timeout", 5*time.Minute, "timeout for --watch")
	return cmd
}

// gitWorkflowStep creates a tui.Step that performs git commit/push.
// This keeps the git package free of TUI dependencies.
func gitWorkflowStep(opts git.WorkflowOpts) tui.Step {
	var stepTitle string
	switch opts.GitMode {
	case "auto":
		stepTitle = "Committing and pushing"
	case "generate":
		stepTitle = "Committing changes"
	default:
		stepTitle = "Staging files"
	}

	return tui.Step{
		Title: stepTitle,
		Run: func() (string, error) {
			repo, err := git.DetectRepo(opts.RepoPath)
			if err != nil {
				return "no git repo detected", nil
			}

			msg := git.FormatCommitMessage(opts.Action, opts.Resource, opts.Details)

			switch opts.GitMode {
			case "auto":
				if err := repo.CommitAndPush(opts.Paths, msg); err != nil {
					return "", err
				}
				return msg, nil
			case "generate":
				if err := repo.Add(opts.Paths...); err != nil {
					return "", err
				}
				if err := repo.Commit(msg); err != nil {
					return "", err
				}
				return msg + " (push manually)", nil
			default:
				if err := repo.Add(opts.Paths...); err != nil {
					return "", fmt.Errorf("staging git changes: %w", err)
				}
				return "staged — commit manually", nil
			}
		},
	}
}
