package scale

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// NewCmd returns the scale command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale namespaces up or down",
		Long:  "Disable ArgoCD auto-sync and scale deployments to 0, or re-enable and scale back up.",
	}

	cmd.AddCommand(newScaleDownCmd())
	cmd.AddCommand(newScaleUpCmd())

	return cmd
}

func newScaleDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [namespace]",
		Short: "Scale down all deployments in a namespace",
		Long: `Disables ArgoCD auto-sync and scales all deployments to 0 in the namespace.
Useful for maintenance or cost-saving on idle namespaces.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns := args[0]
			cfg := config.Get()

			confirmed, _ := tui.Confirm(fmt.Sprintf("Scale down all deployments in namespace %q?", ns))
			if !confirmed {
				fmt.Println("Cancelled")
				return nil
			}

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			fmt.Printf("\n  %s Scaling down namespace %s\n\n", tui.WarningStyle.Render(tui.IconArrow), ns)

			deploys, err := client.ListDeployments(ctx, ns)
			if err != nil {
				return err
			}

			if len(deploys) == 0 {
				fmt.Println(tui.MutedStyle.Render("  No deployments found"))
				return nil
			}

			for _, deploy := range deploys {
				if deploy.ArgoApp != "" {
					fmt.Printf("    %s Disabling auto-sync for %s\n", tui.MutedStyle.Render(tui.IconArrow), deploy.ArgoApp)
					_ = client.DisableArgoAutoSync(ctx, "argocd", deploy.ArgoApp)
				}

				fmt.Printf("    %s Scaling %s to 0\n", tui.MutedStyle.Render(tui.IconArrow), deploy.Name)
				if err := client.ScaleDeployment(ctx, ns, deploy.Name, 0); err != nil {
					fmt.Printf("    %s Failed to scale %s: %v\n", tui.WarningStyle.Render(tui.IconWarn), deploy.Name, err)
				}
			}

			fmt.Printf("\n  %s All deployments in %s scaled down\n", tui.SuccessStyle.Render(tui.IconCheck), ns)
			return nil
		},
	}
}

func newScaleUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [namespace]",
		Short: "Scale up deployments and re-enable ArgoCD sync",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			fmt.Printf("\n  %s Scaling up namespace %s\n\n", tui.InfoStyle.Render(tui.IconArrow), ns)

			deploys, err := client.ListDeployments(ctx, ns)
			if err != nil {
				return err
			}

			if len(deploys) == 0 {
				fmt.Println(tui.MutedStyle.Render("  No deployments found"))
				return nil
			}

			for _, deploy := range deploys {
				if deploy.Replicas == 0 {
					fmt.Printf("    %s Scaling %s to 1\n", tui.MutedStyle.Render(tui.IconArrow), deploy.Name)
					if err := client.ScaleDeployment(ctx, ns, deploy.Name, 1); err != nil {
						fmt.Printf("    %s Failed to scale %s: %v\n", tui.WarningStyle.Render(tui.IconWarn), deploy.Name, err)
					}
				} else {
					fmt.Printf("    %s %s already has %d replicas\n", tui.MutedStyle.Render(tui.IconCheck), deploy.Name, deploy.Replicas)
				}

				if deploy.ArgoApp != "" {
					fmt.Printf("    %s Re-enabling auto-sync for %s\n", tui.MutedStyle.Render(tui.IconArrow), deploy.ArgoApp)
					_ = client.EnableArgoAutoSync(ctx, "argocd", deploy.ArgoApp)
				}
			}

			fmt.Printf("\n  %s All deployments in %s scaled up\n", tui.SuccessStyle.Render(tui.IconCheck), ns)
			return nil
		},
	}
}
