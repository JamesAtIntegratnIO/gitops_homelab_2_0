package kargo

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newWarehousesCmd() *cobra.Command {
	var project string
	var problemsOnly bool

	cmd := &cobra.Command{
		Use:     "warehouses",
		Short:   "List Kargo warehouses and what they last discovered",
		Aliases: []string{"warehouse", "wh"},
		Long: `List Kargo warehouses.

A warehouse that has stopped discovering versions breaks nothing, which is
exactly the problem: everything it feeds is quietly frozen at whatever it last
found, and no app goes unhealthy to say so.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			ctx, cancel := withTimeout()
			defer cancel()

			whs, err := c.ListKargoWarehouses(ctx, project)
			if err != nil {
				return err
			}
			sortByNamespaceName(whs)

			type row struct {
				Project     string `json:"project"`
				Warehouse   string `json:"warehouse"`
				Healthy     string `json:"healthy"`
				Reason      string `json:"reason,omitempty"`
				LastFreight string `json:"lastFreight,omitempty"`
			}
			var data []row
			rows := make([][]string, 0, len(whs))
			var unhealthy int

			for _, w := range whs {
				healthy, reason := conditionOf(w, "Healthy")
				last := str(w.Object, "status", "lastFreightID")

				if healthy == "False" {
					unhealthy++
				}
				if problemsOnly && healthy != "False" {
					continue
				}

				data = append(data, row{
					Project: w.GetNamespace(), Warehouse: w.GetName(),
					Healthy: healthy, Reason: reason, LastFreight: last,
				})
				rows = append(rows, []string{
					w.GetNamespace(), w.GetName(), boolish(healthy),
					truncate(firstNonEmpty(last), 14),
					truncate(firstNonEmpty(reason), 34),
				})
			}

			if tui.PrintStructured(data) {
				return nil
			}

			if len(rows) == 0 {
				fmt.Println("\nNo warehouses match.")
				fmt.Println()
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.SectionHeader("Warehouses"))
			fmt.Println(tui.Table(
				[]string{"PROJECT", "WAREHOUSE", "HEALTHY", "LAST FREIGHT", "REASON"}, rows))
			if unhealthy > 0 {
				fmt.Printf("\n  %d warehouse(s) unhealthy -- version discovery has stopped for those.\n", unhealthy)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Kargo project (namespace); default is all")
	cmd.Flags().BoolVar(&problemsOnly, "problems", false, "Only unhealthy warehouses")
	return cmd
}
