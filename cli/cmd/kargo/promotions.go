package kargo

import (
	"fmt"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newPromotionsCmd() *cobra.Command {
	var project, phaseFilter string
	var failedOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "promotions",
		Short: "List Kargo promotions",
		Long: `List Kargo promotions across every project.

Defaults to showing everything that is not Succeeded, because a successful
promotion is not news -- the reason this command exists is that failures had
nowhere to appear.`,
		Aliases: []string{"promos", "promo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			ctx, cancel := withTimeout()
			defer cancel()

			promos, err := c.ListKargoPromotions(ctx, project)
			if err != nil {
				return err
			}
			sortByNamespaceName(promos)

			var kept []unstructured.Unstructured
			for _, p := range promos {
				ph := phaseOf(p)
				switch {
				case phaseFilter != "" && !strings.EqualFold(ph, phaseFilter):
					continue
				case failedOnly && !isFailure(ph):
					continue
				}
				kept = append(kept, p)
			}

			if limit > 0 && len(kept) > limit {
				kept = kept[:limit]
			}

			type row struct {
				Project   string `json:"project"`
				Stage     string `json:"stage"`
				Promotion string `json:"promotion"`
				Phase     string `json:"phase"`
				Age       string `json:"age"`
				Message   string `json:"message,omitempty"`
			}
			var data []row
			rows := make([][]string, 0, len(kept))
			for _, p := range kept {
				msg := str(p.Object, "status", "message")
				data = append(data, row{
					Project: p.GetNamespace(), Stage: stageOf(p), Promotion: p.GetName(),
					Phase: phaseOf(p), Age: age(p), Message: msg,
				})
				rows = append(rows, []string{
					p.GetNamespace(), stageOf(p), phaseOf(p), age(p),
					truncate(firstNonEmpty(msg), 60),
				})
			}

			if tui.PrintStructured(data) {
				return nil
			}

			if len(rows) == 0 {
				fmt.Println("\nNo promotions match.")
				fmt.Println()
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.SectionHeader("Promotions"))
			fmt.Println(tui.Table(
				[]string{"PROJECT", "STAGE", "PHASE", "AGE", "MESSAGE"}, rows))

			// The count is the point: five broken promotions sitting unreported
			// is the state this whole surface exists to make visible.
			var failing int
			for _, p := range kept {
				if isFailure(phaseOf(p)) {
					failing++
				}
			}
			if failing > 0 {
				fmt.Printf("\n  %d promotion(s) need attention. `hctl kargo describe <stage>` for the step that broke.\n", failing)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Kargo project (namespace); default is all")
	cmd.Flags().StringVar(&phaseFilter, "phase", "", "Only this phase (Pending, Running, Succeeded, Failed, Errored, Aborted)")
	cmd.Flags().BoolVar(&failedOnly, "failed", false, "Only Failed, Errored and Aborted")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of rows (0 = no cap)")

	return cmd
}
