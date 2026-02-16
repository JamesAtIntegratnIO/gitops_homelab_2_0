package secret

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
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
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			data, err := client.GetSecretData(ctx, ns, name)
			if err != nil {
				return err
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

func newSecretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [namespace]",
		Short: "List secrets in a namespace",
		Aliases: []string{"ls"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			secrets, err := client.Clientset.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("listing secrets: %w", err)
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

			fmt.Println(tui.Table([]string{"NAME", "TYPE", "KEYS"}, rows))
			return nil
		},
	}
}
