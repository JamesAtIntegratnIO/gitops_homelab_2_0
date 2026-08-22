package kargo

import (
	"fmt"
	"sort"

	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Summarise delivery state across every Kargo project",
		Long: `One screen answering "is the version pipeline healthy right now".

Counts are per project: stages, how many are unverified, warehouses that have
stopped discovering, and promotions by outcome.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			ctx, cancel := withTimeout()
			defer cancel()

			stages, err := c.ListKargoStages(ctx, "")
			if err != nil {
				return err
			}
			whs, err := c.ListKargoWarehouses(ctx, "")
			if err != nil {
				return err
			}
			promos, err := c.ListKargoPromotions(ctx, "")
			if err != nil {
				return err
			}

			type summary struct {
				Project     string `json:"project"`
				Stages      int    `json:"stages"`
				Unverified  int    `json:"unverified"`
				Warehouses  int    `json:"warehouses"`
				WhUnhealthy int    `json:"warehousesUnhealthy"`
				Running     int    `json:"running"`
				Failed      int    `json:"failed"`
				Errored     int    `json:"errored"`
				Succeeded   int    `json:"succeeded"`
			}
			byProject := map[string]*summary{}
			get := func(ns string) *summary {
				if _, ok := byProject[ns]; !ok {
					byProject[ns] = &summary{Project: ns}
				}
				return byProject[ns]
			}

			for _, s := range stages {
				e := get(s.GetNamespace())
				e.Stages++
				if v, _ := conditionOf(s, "Verified"); v == "False" {
					e.Unverified++
				}
			}
			for _, w := range whs {
				e := get(w.GetNamespace())
				e.Warehouses++
				if h, _ := conditionOf(w, "Healthy"); h == "False" {
					e.WhUnhealthy++
				}
			}
			for _, p := range promos {
				e := get(p.GetNamespace())
				switch phaseOf(p) {
				case "Running", "Pending":
					e.Running++
				case "Failed":
					e.Failed++
				case "Errored", "Aborted":
					e.Errored++
				case "Succeeded":
					e.Succeeded++
				}
			}

			names := make([]string, 0, len(byProject))
			for n := range byProject {
				names = append(names, n)
			}
			sort.Strings(names)

			data := make([]summary, 0, len(names))
			rows := make([][]string, 0, len(names))
			var totalBad int
			for _, n := range names {
				e := byProject[n]
				data = append(data, *e)
				totalBad += e.Failed + e.Errored + e.Unverified + e.WhUnhealthy
				rows = append(rows, []string{
					e.Project,
					fmt.Sprintf("%d", e.Stages),
					fmt.Sprintf("%d", e.Unverified),
					fmt.Sprintf("%d", e.Warehouses),
					fmt.Sprintf("%d", e.WhUnhealthy),
					fmt.Sprintf("%d", e.Running),
					fmt.Sprintf("%d", e.Failed),
					fmt.Sprintf("%d", e.Errored),
				})
			}

			if tui.PrintStructured(data) {
				return nil
			}

			if len(rows) == 0 {
				fmt.Println("\nNo Kargo projects found. Is Kargo installed?")
				fmt.Println()
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.SectionHeader("Kargo delivery"))
			fmt.Println(tui.Table(
				[]string{"PROJECT", "STAGES", "UNVERIF", "WAREHOUSES", "WH BAD", "RUNNING", "FAILED", "ERRORED"}, rows))

			if totalBad == 0 {
				fmt.Println("\n  Nothing needs attention.")
			} else {
				fmt.Printf("\n  %d item(s) need attention:\n", totalBad)
				fmt.Println("    hctl kargo promotions --failed")
				fmt.Println("    hctl kargo stages --problems")
			}
			fmt.Println()
			return nil
		},
	}
	return cmd
}
