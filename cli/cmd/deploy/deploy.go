package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// NewCmd returns the deploy command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy workloads via Score",
		Long: `Deploy workloads to vClusters using Score specification.

Score (score.dev/v1b1) is a platform-agnostic workload spec. hctl translates
Score workloads into platform-native resources (Stakater Application chart,
ExternalSecrets, HTTPRoutes, NetworkPolicies).

Workflow:
  1. hctl deploy init          — scaffold a score.yaml
  2. hctl deploy run           — translate and deploy to the target vCluster
  3. hctl deploy status        — check deployment status
  4. hctl deploy logs          — stream workload logs
  5. hctl deploy remove        — tear down the workload`,
	}

	cmd.AddCommand(newDeployInitCmd())
	cmd.AddCommand(newDeployRunCmd())
	cmd.AddCommand(newDeployStatusCmd())
	cmd.AddCommand(newDeployRemoveCmd())

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
  # HTTP route (creates Gateway API HTTPRoute)
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
  #
  # DNS record
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
		cluster string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Deploy a Score workload to the platform",
		Long: `Translates score.yaml into platform resources and deploys to a vCluster.

Generates:
  - Stakater Application Helm chart values
  - ExternalSecrets for database/service credentials
  - HTTPRoutes for ingress
  - NetworkPolicies for egress

Files are written to workloads/<cluster>/addons/<workload>/ in the gitops repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement Score translation pipeline
			// 1. Parse score.yaml with score-go
			// 2. Validate against score.dev/v1b1 schema
			// 3. Resolve resource DAG
			// 4. Run provisioners (postgres→ExternalSecret, route→HTTPRoute, etc.)
			// 5. Generate Stakater application chart values.yaml
			// 6. Write to workloads/<cluster>/addons/<workload>/
			// 7. Update workloads/<cluster>/addons.yaml
			// 8. Git commit/push
			fmt.Println(tui.WarningStyle.Render("⚠  deploy run is not yet implemented"))
			fmt.Println(tui.DimStyle.Render("  This will translate score.yaml → platform resources using Score provisioners"))
			if dryRun {
				fmt.Println(tui.DimStyle.Render("  (dry-run mode — no files will be written)"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be generated without writing")
	return cmd
}

func newDeployStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [workload]",
		Short: "Check deployment status of a workload",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(tui.WarningStyle.Render("⚠  deploy status is not yet implemented"))
			return nil
		},
	}
}

func newDeployRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [workload]",
		Short: "Remove a deployed workload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(tui.WarningStyle.Render("⚠  deploy remove is not yet implemented"))
			return nil
		},
	}
}
