package kargo

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newStagesCmd() *cobra.Command {
	var project string
	var problemsOnly bool

	cmd := &cobra.Command{
		Use:     "stages",
		Short:   "List Kargo stages with health and verification",
		Aliases: []string{"stage"},
		Long: `List Kargo stages.

The Verified column is the one worth reading. Verification runs after the
merge, so it cannot stop a bad version landing -- but in a promotion chain an
unverified stage silently holds back everything downstream, because Kargo only
offers verified Freight to the next stage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			ctx, cancel := withTimeout()
			defer cancel()

			stages, err := c.ListKargoStages(ctx, project)
			if err != nil {
				return err
			}
			sortByNamespaceName(stages)

			type row struct {
				Project      string `json:"project"`
				Stage        string `json:"stage"`
				Health       string `json:"health"`
				Verified     string `json:"verified"`
				Reason       string `json:"reason,omitempty"`
				LastPromo    string `json:"lastPromotion,omitempty"`
				Verification string `json:"verification,omitempty"`
			}
			var data []row
			rows := make([][]string, 0, len(stages))
			var unhealthy, unverified int

			for _, s := range stages {
				healthy, _ := conditionOf(s, "Healthy")
				verified, vReason := conditionOf(s, "Verified")
				lastPromo := str(s.Object, "status", "lastPromotion", "status", "phase")
				verifyPhase := latestVerificationPhase(s)

				if healthy == "False" {
					unhealthy++
				}
				if verified == "False" {
					unverified++
				}
				if problemsOnly && healthy != "False" && verified != "False" {
					continue
				}

				data = append(data, row{
					Project: s.GetNamespace(), Stage: s.GetName(),
					Health: healthy, Verified: verified, Reason: vReason,
					LastPromo: lastPromo, Verification: verifyPhase,
				})
				rows = append(rows, []string{
					s.GetNamespace(), s.GetName(),
					boolish(healthy), boolish(verified),
					firstNonEmpty(lastPromo), firstNonEmpty(verifyPhase),
					truncate(firstNonEmpty(vReason), 28),
				})
			}

			if tui.PrintStructured(data) {
				return nil
			}

			if len(rows) == 0 {
				fmt.Println("\nNo stages match.")
				fmt.Println()
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.SectionHeader("Stages"))
			fmt.Println(tui.Table(
				[]string{"PROJECT", "STAGE", "HEALTHY", "VERIFIED", "LAST PROMO", "VERIFY", "REASON"}, rows))

			if unhealthy > 0 || unverified > 0 {
				fmt.Printf("\n  %d unhealthy, %d unverified.\n", unhealthy, unverified)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Kargo project (namespace); default is all")
	cmd.Flags().BoolVar(&problemsOnly, "problems", false, "Only stages that are unhealthy or unverified")

	return cmd
}

// latestVerificationPhase reads the most recent verification of the stage's
// current freight. Kargo keeps a stack of them; index 0 is newest.
func latestVerificationPhase(s unstructured.Unstructured) string {
	history, found, _ := unstructured.NestedSlice(s.Object, "status", "freightHistory")
	if !found || len(history) == 0 {
		return ""
	}
	entry, ok := history[0].(map[string]any)
	if !ok {
		return ""
	}
	vh, found, _ := unstructured.NestedSlice(entry, "verificationHistory")
	if !found || len(vh) == 0 {
		return ""
	}
	v, ok := vh[0].(map[string]any)
	if !ok {
		return ""
	}
	phase, _ := v["phase"].(string)
	return phase
}

// boolish renders a condition status as something scannable in a wide table.
func boolish(status string) string {
	switch status {
	case "True":
		return "yes"
	case "False":
		return "NO"
	case "":
		return "-"
	default:
		return status
	}
}
