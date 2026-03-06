package addon

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
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

			client, err := kube.SharedWithConfig(config.Get().KubeContext)
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			app, err := client.GetArgoApp(ctx, "argocd", addonName)
			if err != nil {
				return hcerrors.NewPlatformError("addon %q not found as ArgoCD application: %w", addonName, err)
			}

			syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
			healthStatus := unstr.MustString(app.Object, "status", "health", "status")
			revision := unstr.MustString(app.Object, "status", "sync", "revision")

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(addonName))
			fmt.Printf("  Sync:     %s\n", syncStatus)
			fmt.Printf("  Health:   %s\n", healthStatus)
			fmt.Printf("  Revision: %s\n", revision)
			fmt.Println()

			return nil
		},
	}
}
