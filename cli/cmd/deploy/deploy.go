package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewCmd returns the deploy command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy workloads via Score",
		Long: `Deploy workloads to vClusters using Score specification.

Score (score.dev/v1b1) is a platform-agnostic workload spec. hctl translates
Score workloads into platform-native resources (Stakater Application chart,
ExternalSecrets, HTTPRoutes, Certificates).

Workflow:
  1. hctl deploy init          — scaffold a score.yaml
  2. hctl deploy run           — translate and deploy to the target vCluster
  3. hctl deploy status        — check deployment status
  4. hctl deploy remove        — tear down the workload`,
	}

	cmd.AddCommand(newDeployInitCmd())
	cmd.AddCommand(newDeployRunCmd())
	cmd.AddCommand(newDeployStatusCmd())
	cmd.AddCommand(newDeployRemoveCmd())
	cmd.AddCommand(newDeployListCmd())

	return cmd
}

func newDeployInitCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a score.yaml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cluster == "" {
				cluster = cfg.DefaultCluster
			}

			scorePath := "score.yaml"
			if _, err := os.Stat(scorePath); err == nil {
				confirmed, _ := tui.Confirm("score.yaml already exists. Overwrite?")
				if !confirmed {
					return nil
				}
			}

			// Get workload name from directory
			cwd, _ := os.Getwd()
			workloadName := filepath.Base(cwd)

			domain := cfg.Platform.Domain
			scaffold := fmt.Sprintf(`apiVersion: score.dev/v1b1

metadata:
  name: %s
  annotations:
    # Target vCluster for deployment
    hctl.integratn.tech/cluster: "%s"

containers:
  app:
    image: "."
    variables:
      # Reference resource outputs with ${resources.<name>.<key>}
      # DB_HOST: "${resources.db.host}"
      # DB_PORT: "${resources.db.port}"
      # DB_NAME: "${resources.db.name}"
      # DB_USER: "${resources.db.username}"
      # DB_PASS: "${resources.db.password}"
      PORT: "8080"
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "500m"
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080

service:
  ports:
    http:
      port: 8080
      protocol: TCP

resources:
  # Uncomment resources as needed:
  #
  # PostgreSQL database (credentials from 1Password via ExternalSecret)
  # db:
  #   type: postgres
  #   class: shared
  #
  # HTTP route (creates Gateway API HTTPRoute + TLS Certificate)
  # web:
  #   type: route
  #   params:
  #     host: %s.%s
  #     path: /
  #     port: 8080
  #
  # Persistent volume (NFS via democratic-csi)
  # data:
  #   type: volume
  #   params:
  #     size: 1Gi
  #
  # Redis (credentials from 1Password via ExternalSecret)
  # cache:
  #   type: redis
  #
  # DNS record (handled by external-dns)
  # dns:
  #   type: dns
  #   params:
  #     host: %s.%s
`, workloadName, cluster, workloadName, domain, workloadName, domain)

			if err := os.WriteFile(scorePath, []byte(scaffold), 0o644); err != nil {
				return fmt.Errorf("writing score.yaml: %w", err)
			}

			fmt.Printf("%s Created score.yaml\n", tui.SuccessStyle.Render("✓"))
			fmt.Printf("\n%s\n", tui.DimStyle.Render("Edit score.yaml, then run: hctl deploy run"))
			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (default: from config)")
	return cmd
}

func newDeployRunCmd() *cobra.Command {
	var (
		cluster   string
		dryRun    bool
		scoreFile string
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
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			// Parse Score workload
			workload, err := score.LoadWorkload(scoreFile)
			if err != nil {
				return fmt.Errorf("loading score workload: %w", err)
			}

			fmt.Printf("%s Loaded workload %s\n", tui.SuccessStyle.Render("✓"), tui.TitleStyle.Render(workload.Metadata.Name))

			// Translate to platform resources
			result, err := deploylib.Translate(workload, cluster)
			if err != nil {
				return fmt.Errorf("translating workload: %w", err)
			}

			fmt.Printf("%s Translated for cluster %s (namespace: %s)\n",
				tui.SuccessStyle.Render("✓"), tui.TitleStyle.Render(result.TargetCluster), result.Namespace)

			// Show what will be generated
			if len(workload.Resources) > 0 {
				fmt.Printf("\n  Resources:\n")
				for name, res := range workload.Resources {
					fmt.Printf("    %s %s (%s)\n", tui.SuccessStyle.Render("•"), name, res.Type)
				}
			}

			fmt.Printf("\n  Files:\n")
			for path := range result.Files {
				fmt.Printf("    %s %s\n", tui.SuccessStyle.Render("•"), path)
			}
			fmt.Printf("    %s workloads/%s/addons.yaml\n", tui.SuccessStyle.Render("•"), result.TargetCluster)

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

			// Write files
			writtenPaths, err := deploylib.WriteResult(result, cfg.RepoPath)
			if err != nil {
				return fmt.Errorf("writing files: %w", err)
			}

			fmt.Printf("\n%s Wrote %d files\n", tui.SuccessStyle.Render("✓"), len(writtenPaths))

			// Git operations
			repo, err := git.DetectRepo(cfg.RepoPath)
			if err != nil {
				fmt.Printf("%s Git not available — files written but not committed\n", tui.WarningStyle.Render("⚠"))
				return nil
			}

			switch cfg.GitMode {
			case "auto":
				msg := git.FormatCommitMessage("deploy", workload.Metadata.Name, result.TargetCluster)
				if err := repo.CommitAndPush(writtenPaths, msg); err != nil {
					return fmt.Errorf("git commit/push: %w", err)
				}
				fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))

			case "generate":
				if err := repo.Add(writtenPaths...); err != nil {
					return fmt.Errorf("git add: %w", err)
				}
				msg := git.FormatCommitMessage("deploy", workload.Metadata.Name, result.TargetCluster)
				if err := repo.Commit(msg); err != nil {
					return fmt.Errorf("git commit: %w", err)
				}
				fmt.Printf("%s Committed (push manually)\n", tui.SuccessStyle.Render("✓"))

			case "prompt":
				ok, _ := tui.Confirm("Commit and push changes?")
				if ok {
					msg := git.FormatCommitMessage("deploy", workload.Metadata.Name, result.TargetCluster)
					if err := repo.CommitAndPush(writtenPaths, msg); err != nil {
						return fmt.Errorf("git commit/push: %w", err)
					}
					fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))
				} else {
					if err := repo.Add(writtenPaths...); err == nil {
						fmt.Printf("%s Files staged — commit manually\n", tui.DimStyle.Render("→"))
					}
				}
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will sync the workload automatically."))
			fmt.Printf("%s\n", tui.DimStyle.Render(fmt.Sprintf("Check status: hctl deploy status %s", workload.Metadata.Name)))
			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show generated resources without writing")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	return cmd
}

func newDeployStatusCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "status [workload]",
		Short: "Check deployment status of a workload",
		Long: `Shows the ArgoCD sync and health status of a deployed workload.

If no workload name is given, reads from score.yaml in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			// Determine workload name
			var workloadName string
			if len(args) > 0 {
				workloadName = args[0]
			} else {
				w, err := score.LoadWorkload("score.yaml")
				if err != nil {
					return fmt.Errorf("no workload specified and no score.yaml found: %w", err)
				}
				workloadName = w.Metadata.Name
				if cluster == "" {
					cluster = w.TargetCluster()
				}
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return fmt.Errorf("no cluster specified — use --cluster or set defaultCluster")
			}

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Check ArgoCD app (workload apps are usually in the vCluster's ArgoCD)
			app, err := client.GetArgoApp(ctx, "argocd", workloadName)
			if err != nil {
				// Try with cluster prefix
				app, err = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
				if err != nil {
					return fmt.Errorf("ArgoCD application not found for %q", workloadName)
				}
			}

			syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
			healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
			revision, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "revision")

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(workloadName))
			fmt.Printf("  Cluster:  %s\n", cluster)

			statusStr := fmt.Sprintf("%s/%s", syncStatus, healthStatus)
			if syncStatus == "Synced" && healthStatus == "Healthy" {
				fmt.Printf("  Status:   %s\n", tui.SuccessStyle.Render(statusStr))
			} else {
				fmt.Printf("  Status:   %s\n", tui.WarningStyle.Render(statusStr))
			}
			if revision != "" {
				fmt.Printf("  Revision: %s\n", tui.DimStyle.Render(revision))
			}

			// Check pods
			namespace := cluster
			pods, err := client.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
			if err == nil && len(pods) > 0 {
				fmt.Printf("\n  Pods:\n")
				for _, p := range pods {
					status := tui.SuccessStyle.Render(p.Phase)
					if p.Phase != "Running" || p.ReadyContainers < p.TotalContainers {
						status = tui.WarningStyle.Render(p.Phase)
					}
					fmt.Printf("    %s  %d/%d  %s\n", p.Name, p.ReadyContainers, p.TotalContainers, status)
				}
			}

			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}

func newDeployRemoveCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "remove [workload]",
		Short: "Remove a deployed workload",
		Long: `Removes a workload from the platform by deleting its entry from addons.yaml
and removing its values directory. ArgoCD will clean up the resources on next sync.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workloadName := args[0]
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return fmt.Errorf("no cluster specified — use --cluster or set defaultCluster")
			}

			// Confirm removal
			if cfg.Interactive {
				ok, _ := tui.Confirm(fmt.Sprintf("Remove workload %q from cluster %q?", workloadName, cluster))
				if !ok {
					fmt.Println(tui.DimStyle.Render("Cancelled"))
					return nil
				}
			}

			removedPaths, err := deploylib.RemoveWorkload(cfg.RepoPath, cluster, workloadName)
			if err != nil {
				return err
			}

			fmt.Printf("%s Removed workload %s from %s\n",
				tui.SuccessStyle.Render("✓"), workloadName, cluster)

			// Git operations
			repo, err := git.DetectRepo(cfg.RepoPath)
			if err != nil {
				return nil
			}

			switch cfg.GitMode {
			case "auto":
				msg := git.FormatCommitMessage("remove", workloadName, cluster)
				if err := repo.CommitAndPush(removedPaths, msg); err != nil {
					return fmt.Errorf("git commit/push: %w", err)
				}
				fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))

			case "generate":
				if err := repo.Add(removedPaths...); err == nil {
					msg := git.FormatCommitMessage("remove", workloadName, cluster)
					_ = repo.Commit(msg)
				}
				fmt.Printf("%s Committed (push manually)\n", tui.SuccessStyle.Render("✓"))

			case "prompt":
				ok, _ := tui.Confirm("Commit and push removal?")
				if ok {
					msg := git.FormatCommitMessage("remove", workloadName, cluster)
					if err := repo.CommitAndPush(removedPaths, msg); err != nil {
						return fmt.Errorf("git commit/push: %w", err)
					}
					fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))
				}
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will remove the workload on next sync."))
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}

func newDeployListCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployed workloads for a cluster",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return fmt.Errorf("no cluster specified — use --cluster or set defaultCluster")
			}

			workloads, err := deploylib.ListWorkloads(cfg.RepoPath, cluster)
			if err != nil {
				return fmt.Errorf("reading workloads: %w", err)
			}

			if len(workloads) == 0 {
				fmt.Println(tui.DimStyle.Render("No workloads deployed to " + cluster))
				return nil
			}

			var rows [][]string
			for _, name := range workloads {
				rows = append(rows, []string{name, cluster})
			}

			fmt.Println(tui.Table([]string{"WORKLOAD", "CLUSTER"}, rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}
