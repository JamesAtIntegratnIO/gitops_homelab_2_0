package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runDiagnose(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	var result *platform.DiagnosticResult

	client, err := kube.SharedWithConfig(config.Get().KubeContext)
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	_, err = tui.Spin("Diagnosing "+name, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err = platform.DiagnoseVCluster(ctx, client, cfg.Platform.PlatformNamespace, name)
		return "", err
	})
	if err != nil {
		return hcerrors.NewPlatformError("diagnosing %s: %w", name, err)
	}

	// Structured output or bundle export
	if tui.IsStructured() || bundlePath != "" {
		bundle := map[string]interface{}{
			"resource":  name,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"steps":     result.Steps,
		}
		if bundlePath != "" {
			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return hcerrors.NewPlatformError("marshaling diagnostic bundle: %w", err)
			}
			if writeErr := os.WriteFile(bundlePath, data, 0o644); writeErr != nil {
				return hcerrors.NewPlatformError("writing bundle: %w", writeErr)
			}
			fmt.Printf("%s Diagnostic bundle written to %s\n",
				tui.SuccessStyle.Render(tui.IconCheck), bundlePath)
			if !tui.IsStructured() {
				return nil // don't print text output if we wrote a bundle
			}
		}
		return tui.RenderOutput(bundle, "")
	}

	fmt.Printf("\n  %s\n", name)
	for i, step := range result.Steps {
		isLast := i == len(result.Steps)-1
		fmt.Println(tui.TreeNode(
			fmt.Sprintf("%-15s", step.Name),
			tui.DiagIcon(int(step.Status)),
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
}
