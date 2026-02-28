package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
  3. hctl deploy render        — preview rendered manifests
  4. hctl deploy diff          — compare rendered vs on-disk
  5. hctl deploy status        — check deployment status
  6. hctl deploy remove        — tear down the workload`,
	}

	cmd.AddCommand(newDeployInitCmd())
	cmd.AddCommand(newDeployRunCmd())
	cmd.AddCommand(newDeployRenderCmd())
	cmd.AddCommand(newDeployDiffCmd())
	cmd.AddCommand(newDeployStatusCmd())
	cmd.AddCommand(newDeployRemoveCmd())
	cmd.AddCommand(newDeployListCmd())

	return cmd
}

func newDeployInitCmd() *cobra.Command {
	var (
		cluster  string
		template string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a score.yaml in the current directory",
		Long: `Creates a score.yaml from a template. Templates provide pre-configured
resource combinations for common workload types.

Templates:
  web      Web application with HTTP route, TLS, and health checks (default)
  api      API service with HTTP route, database, and health checks
  worker   Background worker with no ingress, optional database
  cron     Minimal container spec for cron/batch jobs`,
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

			cwd, _ := os.Getwd()
			workloadName := filepath.Base(cwd)
			domain := cfg.Platform.Domain

			scaffold := generateScoreTemplate(template, workloadName, cluster, domain)

			if err := os.WriteFile(scorePath, []byte(scaffold), 0o644); err != nil {
				return fmt.Errorf("writing score.yaml: %w", err)
			}

			fmt.Printf("%s Created score.yaml (template: %s)\n",
				tui.SuccessStyle.Render(tui.IconCheck), template)
			fmt.Printf("\n%s\n", tui.DimStyle.Render("Edit score.yaml, then run: hctl deploy run"))
			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (default: from config)")
	cmd.Flags().StringVarP(&template, "template", "t", "web", "workload template: web, api, worker, cron")
	return cmd
}

// generateScoreTemplate returns a Score spec scaffold for the given template type.
func generateScoreTemplate(tmpl, name, cluster, domain string) string {
	switch tmpl {
	case "api":
		return fmt.Sprintf(`apiVersion: score.dev/v1b1

metadata:
  name: %s
  annotations:
    hctl.integratn.tech/cluster: "%s"

containers:
  app:
    image: "."
    variables:
      PORT: "8080"
      DB_HOST: "${resources.db.host}"
      DB_PORT: "${resources.db.port}"
      DB_NAME: "${resources.db.name}"
      DB_USER: "${resources.db.username}"
      DB_PASS: "${resources.db.password}"
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
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
  db:
    type: postgres
    class: shared

  web:
    type: route
    params:
      host: %s.%s
      path: /
      port: 8080
`, name, cluster, name, domain)

	case "worker":
		return fmt.Sprintf(`apiVersion: score.dev/v1b1

metadata:
  name: %s
  annotations:
    hctl.integratn.tech/cluster: "%s"

containers:
  worker:
    image: "."
    variables:
      # DB_HOST: "${resources.db.host}"
      # REDIS_HOST: "${resources.cache.host}"
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "500m"
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080

service:
  ports:
    health:
      port: 8080
      protocol: TCP

resources:
  # Uncomment as needed:
  # db:
  #   type: postgres
  #   class: shared
  # cache:
  #   type: redis
`, name, cluster)

	case "cron":
		return fmt.Sprintf(`apiVersion: score.dev/v1b1

metadata:
  name: %s
  annotations:
    hctl.integratn.tech/cluster: "%s"

containers:
  job:
    image: "."
    command: ["/bin/sh", "-c"]
    args: ["echo 'hello world'"]
    resources:
      requests:
        memory: "64Mi"
        cpu: "50m"
      limits:
        memory: "256Mi"
        cpu: "250m"

# No service or route — batch jobs don't serve traffic
resources: {}
`, name, cluster)

	default: // "web"
		return fmt.Sprintf(`apiVersion: score.dev/v1b1

metadata:
  name: %s
  annotations:
    hctl.integratn.tech/cluster: "%s"

containers:
  app:
    image: "."
    variables:
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
  web:
    type: route
    params:
      host: %s.%s
      path: /
      port: 8080

  # Uncomment to add more resources:
  # db:
  #   type: postgres
  #   class: shared
  # cache:
  #   type: redis
  # data:
  #   type: volume
  #   params:
  #     size: 1Gi
`, name, cluster, name, domain)
	}
}

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
				return fmt.Errorf("repo path not set — run 'hctl init'")
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
						r, err := deploylib.Translate(workload, cluster)
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
			gitStep := git.HandleGitWorkflowStep(git.WorkflowOpts{
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
					return fmt.Errorf("deploy failed at %q: %w", r.Title, r.Err)
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
			client, cErr := kube.NewClient(cfg.KubeContext)
			if cErr != nil {
				return fmt.Errorf("connecting to cluster for watch: %w", cErr)
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
					syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
					healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
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
					return fmt.Errorf("timeout waiting for sync after %s", watchTimeout)
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

func newDeployRenderCmd() *cobra.Command {
	var (
		cluster   string
		scoreFile string
	)
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render Score workload to Kubernetes manifests",
		Long: `Translates score.yaml and prints the generated platform resources to stdout.
No files are written and no git operations are performed.

Useful for reviewing what will be generated before running 'hctl deploy run'.
Supports --output json/yaml for machine-readable output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workload, err := score.LoadWorkload(scoreFile)
			if err != nil {
				return fmt.Errorf("loading score workload: %w", err)
			}

			result, err := deploylib.Translate(workload, cluster)
			if err != nil {
				return fmt.Errorf("translating workload: %w", err)
			}

			// Structured output: emit the full translation result
			if tui.IsStructured() {
				renderData := map[string]interface{}{
					"workload":       result.WorkloadName,
					"cluster":        result.TargetCluster,
					"namespace":      result.Namespace,
					"stakaterValues": result.StakaterValues,
					"addonsEntry":    result.AddonsEntry,
					"files":          map[string]string{},
				}
				filesMap := renderData["files"].(map[string]string)
				for path, data := range result.Files {
					filesMap[path] = string(data)
				}
				return tui.RenderOutput(renderData, "")
			}

			// Text output: print each file with a header
			for path, data := range result.Files {
				fmt.Printf("%s\n", tui.TitleStyle.Render("# "+path))
				fmt.Println(string(data))
			}

			// Show addons.yaml entry
			fmt.Printf("%s\n", tui.TitleStyle.Render("# addons.yaml entry"))
			entry, _ := yaml.Marshal(map[string]interface{}{result.WorkloadName: result.AddonsEntry})
			fmt.Println(string(entry))

			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	return cmd
}

func newDeployDiffCmd() *cobra.Command {
	var (
		cluster   string
		scoreFile string
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between rendered and deployed manifests",
		Long: `Translates score.yaml and compares the output against what is currently
on disk in the gitops repo. Shows a unified diff for each changed file.

Exit codes: 0 = no changes, 1 = error, 2 = changes detected.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			workload, err := score.LoadWorkload(scoreFile)
			if err != nil {
				return fmt.Errorf("loading score workload: %w", err)
			}

			result, err := deploylib.Translate(workload, cluster)
			if err != nil {
				return fmt.Errorf("translating workload: %w", err)
			}

			hasChanges := false

			// Compare each rendered file against what's on disk
			for relPath, newData := range result.Files {
				absPath := filepath.Join(cfg.RepoPath, relPath)
				existing, readErr := os.ReadFile(absPath)
				if readErr != nil {
					// File doesn't exist yet — show as new
					fmt.Printf("%s %s\n", tui.SuccessStyle.Render("+ new file:"), relPath)
					fmt.Println(string(newData))
					hasChanges = true
					continue
				}

				if string(existing) == string(newData) {
					fmt.Printf("%s %s\n", tui.DimStyle.Render("  unchanged:"), relPath)
					continue
				}

				hasChanges = true
				fmt.Printf("%s %s\n", tui.WarningStyle.Render("~ modified:"), relPath)
				printUnifiedDiff(relPath, string(existing), string(newData))
			}

			// Check addons.yaml for changes
			addonsPath := filepath.Join(cfg.RepoPath, "workloads", result.TargetCluster, "addons.yaml")
			if existingAddons, readErr := os.ReadFile(addonsPath); readErr == nil {
				var existingMap map[string]interface{}
				if yaml.Unmarshal(existingAddons, &existingMap) == nil {
					if existing, ok := existingMap[result.WorkloadName]; ok {
						existingYAML, _ := yaml.Marshal(existing)
						newYAML, _ := yaml.Marshal(result.AddonsEntry)
						if string(existingYAML) != string(newYAML) {
							hasChanges = true
							relPath := filepath.Join("workloads", result.TargetCluster, "addons.yaml")
							fmt.Printf("%s %s (entry: %s)\n", tui.WarningStyle.Render("~ modified:"), relPath, result.WorkloadName)
							printUnifiedDiff(relPath, string(existingYAML), string(newYAML))
						}
					} else {
						hasChanges = true
						fmt.Printf("%s addons.yaml (new entry: %s)\n", tui.SuccessStyle.Render("+ new:"), result.WorkloadName)
					}
				}
			} else {
				hasChanges = true
				fmt.Printf("%s addons.yaml (new file)\n", tui.SuccessStyle.Render("+ new:"))
			}

			if !hasChanges {
				fmt.Println(tui.DimStyle.Render("No changes detected"))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	return cmd
}

// printUnifiedDiff prints a simple line-by-line diff between two strings.
func printUnifiedDiff(path, old, new string) {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Simple line-by-line comparison (not a true unified diff, but useful)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if i < len(oldLines) {
				fmt.Printf("  %s\n", tui.ErrorStyle.Render("- "+oldLine))
			}
			if i < len(newLines) {
				fmt.Printf("  %s\n", tui.SuccessStyle.Render("+ "+newLine))
			}
		}
	}
	fmt.Println()
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
				tui.SuccessStyle.Render(tui.IconCheck), workloadName, cluster)

			// Git operations
			if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
				RepoPath:      cfg.RepoPath,
				Paths:         removedPaths,
				Action:        "remove",
				Resource:      workloadName,
				Details:       cluster,
				GitMode:       cfg.GitMode,
				Interactive:   cfg.Interactive,
				ConfirmPrompt: "Commit and push removal?",
			}); err != nil {
				return err
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

			_, err = tui.InteractiveTable(tui.InteractiveTableConfig{
				Title:   "Workloads (" + cluster + ")",
				Headers: []string{"WORKLOAD", "CLUSTER"},
				Rows:    rows,
				OnSelect: func(row []string, index int) string {
					if len(row) == 0 {
						return ""
					}
					workloadName := row[0]

					client, cErr := kube.NewClient(cfg.KubeContext)
					if cErr != nil {
						return tui.ErrorStyle.Render("Cannot connect: " + cErr.Error())
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					var sb strings.Builder
					sb.WriteString(tui.HeaderStyle.Render("Workload: "+workloadName) + "\n\n")

					// ArgoCD status
					app, aErr := client.GetArgoApp(ctx, "argocd", workloadName)
					if aErr != nil {
						app, aErr = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
					}
					if aErr == nil {
						syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
						healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
						statusStr := syncStatus + "/" + healthStatus
						if syncStatus == "Synced" && healthStatus == "Healthy" {
							sb.WriteString("  Status: " + tui.SuccessStyle.Render(statusStr) + "\n")
						} else {
							sb.WriteString("  Status: " + tui.WarningStyle.Render(statusStr) + "\n")
						}
					} else {
						sb.WriteString("  Status: " + tui.DimStyle.Render("not found in ArgoCD") + "\n")
					}

					// Pods
					pods, pErr := client.ListPods(ctx, cluster, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
					if pErr == nil && len(pods) > 0 {
						sb.WriteString("\n  Pods:\n")
						for _, p := range pods {
							status := tui.SuccessStyle.Render(p.Phase)
							if p.Phase != "Running" || p.ReadyContainers < p.TotalContainers {
								status = tui.WarningStyle.Render(p.Phase)
							}
							sb.WriteString(fmt.Sprintf("    %s  %d/%d  %s\n", p.Name, p.ReadyContainers, p.TotalContainers, status))
						}
					}

					return sb.String()
				},
			})
			return err
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}
