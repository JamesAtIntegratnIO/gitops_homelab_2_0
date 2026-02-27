package scale

import (
	"fmt"
	"os/exec"
	"strings"

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

			confirmed, _ := tui.Confirm(fmt.Sprintf("Scale down all deployments in namespace %q?", ns))
			if !confirmed {
				fmt.Println("Cancelled")
				return nil
			}

			fmt.Printf("\n  %s Scaling down namespace %s\n\n", tui.WarningStyle.Render(tui.IconArrow), ns)

			// Get all deployments
			out, err := kubectl("get", "deploy", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
			if err != nil {
				return fmt.Errorf("listing deployments: %w", err)
			}

			deploys := strings.Fields(strings.TrimSpace(out))
			if len(deploys) == 0 {
				fmt.Println(tui.MutedStyle.Render("  No deployments found"))
				return nil
			}

			for _, deploy := range deploys {
				// Get ArgoCD tracking annotation
				tracking, _ := kubectl("get", "deploy", deploy, "-n", ns,
					"-o", "jsonpath={.metadata.annotations.argocd\\.argoproj\\.io/tracking-id}")

				if tracking != "" {
					appName := strings.SplitN(tracking, ":", 2)[0]
					fmt.Printf("    %s Disabling auto-sync for %s\n", tui.MutedStyle.Render(tui.IconArrow), appName)
					_, _ = kubectl("patch", "application", appName, "-n", "argocd",
						"--type", "merge", "-p", `{"spec":{"syncPolicy":null}}`)
				}

				fmt.Printf("    %s Scaling %s to 0\n", tui.MutedStyle.Render(tui.IconArrow), deploy)
				_, err := kubectl("scale", "deploy", deploy, "-n", ns, "--replicas=0")
				if err != nil {
					fmt.Printf("    %s Failed to scale %s: %v\n", tui.WarningStyle.Render(tui.IconWarn), deploy, err)
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

			fmt.Printf("\n  %s Scaling up namespace %s\n\n", tui.InfoStyle.Render(tui.IconArrow), ns)

			out, err := kubectl("get", "deploy", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
			if err != nil {
				return fmt.Errorf("listing deployments: %w", err)
			}

			deploys := strings.Fields(strings.TrimSpace(out))
			if len(deploys) == 0 {
				fmt.Println(tui.MutedStyle.Render("  No deployments found"))
				return nil
			}

			for _, deploy := range deploys {
				// Check current replicas
				currentStr, _ := kubectl("get", "deploy", deploy, "-n", ns,
					"-o", "jsonpath={.spec.replicas}")
				if currentStr == "0" {
					fmt.Printf("    %s Scaling %s to 1\n", tui.MutedStyle.Render(tui.IconArrow), deploy)
					_, _ = kubectl("scale", "deploy", deploy, "-n", ns, "--replicas=1")
				} else {
					fmt.Printf("    %s %s already has %s replicas\n", tui.MutedStyle.Render(tui.IconCheck), deploy, currentStr)
				}

				// Re-enable ArgoCD sync
				tracking, _ := kubectl("get", "deploy", deploy, "-n", ns,
					"-o", "jsonpath={.metadata.annotations.argocd\\.argoproj\\.io/tracking-id}")
				if tracking != "" {
					appName := strings.SplitN(tracking, ":", 2)[0]
					fmt.Printf("    %s Re-enabling auto-sync for %s\n", tui.MutedStyle.Render(tui.IconArrow), appName)
					_, _ = kubectl("patch", "application", appName, "-n", "argocd",
						"--type", "merge", "-p",
						`{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}`)
				}
			}

			fmt.Printf("\n  %s All deployments in %s scaled up\n", tui.SuccessStyle.Render(tui.IconCheck), ns)
			return nil
		},
	}
}

func kubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
