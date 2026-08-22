package kargo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newDescribeCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "describe <stage>",
		Short: "Explain a stage's state and why its last promotion failed",
		Args:  cobra.ExactArgs(1),
		Long: `Describe a Kargo stage: its conditions, its current freight, and the
step-level error from its most recent promotion.

This is the command that surfaces the actual failure text. A target whose
parse path no longer resolves fails with something like

  error evaluating expression "spec.jobTemplate...": cannot fetch spec from <nil>

which appears in the Promotion's status message and nowhere else -- not in a
metric, not in an ArgoCD app, not in any log a human would think to read.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			c, err := client()
			if err != nil {
				return err
			}
			ctx, cancel := withTimeout()
			defer cancel()

			// Resolve the project when not given, so `describe cert-manager`
			// works without the caller remembering which project it lives in.
			ns := project
			if ns == "" {
				stages, err := c.ListKargoStages(ctx, "")
				if err != nil {
					return err
				}
				var matches []string
				for _, s := range stages {
					if s.GetName() == name {
						matches = append(matches, s.GetNamespace())
					}
				}
				switch len(matches) {
				case 0:
					return fmt.Errorf("no stage named %q in any project", name)
				case 1:
					ns = matches[0]
				default:
					sort.Strings(matches)
					return fmt.Errorf("stage %q exists in %s; pass --project to choose",
						name, strings.Join(matches, ", "))
				}
			}

			stage, err := c.GetKargoStage(ctx, ns, name)
			if err != nil {
				return err
			}

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(ns+"/"+name))

			healthy, hReason := conditionOf(*stage, "Healthy")
			verified, vReason := conditionOf(*stage, "Verified")
			ready, rReason := conditionOf(*stage, "Ready")
			fmt.Printf("  Ready:     %s %s\n", boolish(ready), dim(rReason))
			fmt.Printf("  Healthy:   %s %s\n", boolish(healthy), dim(hReason))
			fmt.Printf("  Verified:  %s %s\n", boolish(verified), dim(vReason))

			if fs := str(stage.Object, "status", "freightSummary"); fs != "" {
				fmt.Printf("  Freight:   %s\n", fs)
			}
			if vp := latestVerificationPhase(*stage); vp != "" {
				fmt.Printf("  Verify:    %s\n", vp)
			}
			fmt.Println()

			// The most recent promotion carries the step-level error.
			promos, err := c.ListKargoPromotions(ctx, ns)
			if err != nil {
				return err
			}
			var latest *unstructured.Unstructured
			for i := range promos {
				if stageOf(promos[i]) != name {
					continue
				}
				if latest == nil || promos[i].GetCreationTimestamp().Time.After(latest.GetCreationTimestamp().Time) {
					latest = &promos[i]
				}
			}
			if latest == nil {
				fmt.Println("  No promotions recorded for this stage.")
				fmt.Println()
				return nil
			}

			fmt.Printf("%s\n\n", tui.SectionHeader("Most recent promotion"))
			fmt.Printf("  Name:   %s\n", latest.GetName())
			fmt.Printf("  Phase:  %s\n", phaseOf(*latest))
			fmt.Printf("  Age:    %s\n", age(*latest))
			if msg := str(latest.Object, "status", "message"); msg != "" {
				fmt.Printf("\n  %s\n", tui.SectionHeader("Message"))
				for _, line := range strings.Split(strings.TrimRight(msg, "\n"), "\n") {
					fmt.Printf("    %s\n", line)
				}
			}

			// Per-step state, which is where "which step" actually lives.
			if steps := stepStates(*latest); len(steps) > 0 {
				fmt.Printf("\n  %s\n\n", tui.SectionHeader("Steps"))
				fmt.Println(tui.Table([]string{"STEP", "USES", "STATUS", "MESSAGE"}, steps))
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Kargo project (namespace); inferred when unambiguous")
	return cmd
}

func stepStates(p unstructured.Unstructured) [][]string {
	steps, found, _ := unstructured.NestedSlice(p.Object, "status", "stepExecutionMetadata")
	if !found {
		return nil
	}
	specSteps, _, _ := unstructured.NestedSlice(p.Object, "spec", "steps")

	rows := make([][]string, 0, len(steps))
	for i, s := range steps {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		alias, _ := m["alias"].(string)
		status, _ := m["status"].(string)
		msg, _ := m["message"].(string)

		uses := ""
		if i < len(specSteps) {
			if sm, ok := specSteps[i].(map[string]any); ok {
				uses, _ = sm["uses"].(string)
			}
		}
		rows = append(rows, []string{
			firstNonEmpty(alias, fmt.Sprintf("step-%d", i+1)),
			firstNonEmpty(uses),
			firstNonEmpty(status),
			truncate(firstNonEmpty(msg), 70),
		})
	}
	return rows
}

func dim(s string) string {
	if s == "" {
		return ""
	}
	return "(" + s + ")"
}
