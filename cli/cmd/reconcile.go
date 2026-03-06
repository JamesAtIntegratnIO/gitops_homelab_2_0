package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runReconcile(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.SetManualReconciliationLabel(ctx, kube.VClusterOrchestratorV2GVR, cfg.Platform.PlatformNamespace, name)
	if err != nil {
		return hcerrors.NewPlatformError("setting reconciliation label: %w", err)
	}

	fmt.Printf("  %s Set kratix.io/manual-reconciliation=true on %s\n", tui.SuccessStyle.Render(tui.IconCheck), name)
	return nil
}
