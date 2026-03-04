package addon

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
	"github.com/spf13/cobra"
)

func newAddonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [addon]",
		Short: "Check addon health and sync status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addonName := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			app, err := client.GetArgoApp(ctx, "argocd", addonName)
			if err != nil {
				return fmt.Errorf("addon %q not found as ArgoCD application: %w", addonName, err)
			}

			syncStatus, _, _ := unstr.NestedString(app.Object, "status", "sync", "status")
			healthStatus, _, _ := unstr.NestedString(app.Object, "status", "health", "status")
			revision, _, _ := unstr.NestedString(app.Object, "status", "sync", "revision")

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(addonName))
			fmt.Printf("  Sync:     %s\n", syncStatus)
			fmt.Printf("  Health:   %s\n", healthStatus)
			fmt.Printf("  Revision: %s\n", revision)
			fmt.Println()

			return nil
		},
	}
}
