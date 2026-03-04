package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

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
				confirmed, confirmErr := tui.Confirm("score.yaml already exists. Overwrite?")
				if confirmErr != nil {
					return fmt.Errorf("confirming overwrite: %w", confirmErr)
				}
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
