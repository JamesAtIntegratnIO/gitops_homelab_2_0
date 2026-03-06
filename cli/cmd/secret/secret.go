package secret

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCmd returns the secret command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Inspect Kubernetes secrets",
		Long:  "Decode and display secret data from the cluster.",
	}

	cmd.AddCommand(newSecretGetCmd())
	cmd.AddCommand(newSecretListCmd())

	return cmd
}

func newSecretGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [namespace] [name]",
		Short: "Decode and display secret data",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name := args[0], args[1]

			client, err := kube.SharedWithConfig(config.Get().KubeContext)
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			data, err := client.GetSecretData(ctx, ns, name)
			if err != nil {
				return hcerrors.NewPlatformError("fetching secret %s/%s: %w", ns, name, err)
			}

			fmt.Printf("\n%s %s/%s\n\n", tui.TitleStyle.Render("Secret"), ns, name)
			for k, v := range data {
				fmt.Printf("  %s: %s\n", tui.HeaderStyle.Render(k), string(v))
			}
			fmt.Println()
			return nil
		},
	}
}

// newSecretListCmd returns a command that lists secrets in a namespace.
//
// NOTE: Directly accesses client.Clientset for secret List operations not
// covered by the KubeClient interface. The interface is scoped to platform
// lifecycle (vClusters, ArgoCD); general-purpose secret enumeration with
// interactive TUI selection is CLI-specific and doesn't belong there.
func newSecretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [namespace]",
		Short: "List secrets in a namespace",
		Aliases: []string{"ls"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns := args[0]

			client, err := kube.SharedWithConfig(config.Get().KubeContext)
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Direct Clientset access — see function doc comment.
			secrets, err := client.Clientset.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return hcerrors.NewPlatformError("listing secrets: %w", err)
			}

			if len(secrets.Items) == 0 {
				fmt.Println(tui.DimStyle.Render("No secrets found"))
				return nil
			}

			var rows [][]string
			for _, s := range secrets.Items {
				rows = append(rows, []string{
					s.Name,
					string(s.Type),
					fmt.Sprintf("%d", len(s.Data)),
				})
			}

			// Interactive table: enter to view secret data
			action, err := tui.InteractiveTable(tui.InteractiveTableConfig{
				Title:   fmt.Sprintf("Secrets in %s", ns),
				Headers: []string{"NAME", "TYPE", "KEYS"},
				Rows:    rows,
				OnSelect: func(row []string, index int) string {
					secretName := row[0]
					ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel2()
					data, err := client.GetSecretData(ctx2, ns, secretName)
					if err != nil {
						return tui.ErrorStyle.Render("Error: " + err.Error())
					}
					var sb strings.Builder
					sb.WriteString(tui.TitleStyle.Render(secretName) + "\n")
					for k, v := range data {
						sb.WriteString(fmt.Sprintf("  %s: %s\n", tui.HeaderStyle.Render(k), string(v)))
					}
					return sb.String()
				},
			})
			_ = action
			return err
		},
	}
}
