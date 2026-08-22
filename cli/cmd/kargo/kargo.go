// Package kargo surfaces Kargo's promotion state on the command line.
//
// Kargo exposes controller-runtime counters and nothing about promotions,
// freight or stages, so before this existed the only way to find a failed
// promotion was to open the UI and look. These commands read the CRDs
// directly, which means they work whether or not the metrics pipeline does.
package kargo

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the kargo command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kargo",
		Short: "Inspect Kargo promotion state",
		Long: `Inspect Kargo promotions, stages and warehouses.

Kargo keeps every promotion attempt as an object, so the interesting question
is rarely "what is running" -- it is "what failed and never told anyone".

  hctl kargo status              what each project looks like right now
  hctl kargo promotions          promotion history, filterable by phase
  hctl kargo stages              current freight, health and verification
  hctl kargo describe <stage>    why a stage's last promotion failed
  hctl kargo warehouses          what is discovering versions, and what is not`,
	}

	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newPromotionsCmd())
	cmd.AddCommand(newStagesCmd())
	cmd.AddCommand(newDescribeCmd())
	cmd.AddCommand(newWarehousesCmd())

	return cmd
}
