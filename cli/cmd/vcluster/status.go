package vcluster

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show vCluster lifecycle status",
		Long:  "Walks the full resource lifecycle chain and displays status at each stage.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := platform.DiagnoseVCluster(ctx, client, cfg.Platform.PlatformNamespace, name)
			if err != nil {
				return err
			}

			fmt.Printf("\n  %s\n", tui.TitleStyle.Render(name))
			for i, step := range result.Steps {
				isLast := i == len(result.Steps)-1
				fmt.Println(tui.TreeNode(
					fmt.Sprintf("%-15s", step.Name),
					step.Status.String(),
					step.Message,
					isLast,
				))
				if step.Details != "" {
					indent := "  │   "
					if isLast {
						indent = "      "
					}
					fmt.Printf("%s%s\n", indent, tui.DimStyle.Render(step.Details))
				}
			}
			fmt.Println()

			return nil
		},
	}
}
