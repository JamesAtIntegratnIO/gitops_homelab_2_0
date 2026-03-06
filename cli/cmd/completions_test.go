package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteVClusterNames_RejectsExtraArgs(t *testing.T) {
	cmd := &cobra.Command{}
	// When args already present, should return no completions
	names, directive := completeVClusterNames(cmd, []string{"already-have-one"}, "")
	if names != nil {
		t.Errorf("expected nil names when args present, got %v", names)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteWorkloadNames_RejectsExtraArgs(t *testing.T) {
	cmd := &cobra.Command{}
	names, directive := completeWorkloadNames(cmd, []string{"already-have-one"}, "")
	if names != nil {
		t.Errorf("expected nil names when args present, got %v", names)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestRegisterCompletions_SetsValidArgsFunction(t *testing.T) {
	diagnoseCmd := &cobra.Command{Use: "diagnose"}
	reconcileCmd := &cobra.Command{Use: "reconcile"}
	traceCmd := &cobra.Command{Use: "trace"}
	upCmd := &cobra.Command{Use: "up"}
	downCmd := &cobra.Command{Use: "down"}
	logsCmd := &cobra.Command{Use: "logs"}
	openCmd := &cobra.Command{Use: "open"}

	registerCompletions(diagnoseCmd, reconcileCmd, traceCmd, upCmd, downCmd, logsCmd, openCmd)

	cmds := []*cobra.Command{diagnoseCmd, reconcileCmd, traceCmd, upCmd, downCmd, logsCmd, openCmd}
	for _, c := range cmds {
		if c.ValidArgsFunction == nil {
			t.Errorf("ValidArgsFunction should be set for %q", c.Use)
		}
	}
}
