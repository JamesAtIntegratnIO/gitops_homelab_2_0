package cmd

import (
	"testing"
	"time"
)

// Tests for the business logic previously in this file have been moved to
// internal/platform/workload_test.go alongside the extracted functions.
// Below are cmd-layer tests for command construction, flags, and validation.

func TestNewUpCmd(t *testing.T) {
	cmd := newUpCmd()
	if cmd.Use != "up [workload]" {
		t.Errorf("Use = %q, want 'up [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewUpCmd_ReplicasFlag(t *testing.T) {
	cmd := newUpCmd()
	flag := cmd.Flags().Lookup("replicas")
	if flag == nil {
		t.Fatal("missing --replicas flag")
	}
	if flag.Shorthand != "r" {
		t.Errorf("replicas shorthand = %q, want 'r'", flag.Shorthand)
	}
	if flag.DefValue != "1" {
		t.Errorf("replicas default = %q, want '1'", flag.DefValue)
	}
}

func TestNewUpCmd_AcceptsMaxOneArg(t *testing.T) {
	cmd := newUpCmd()
	if cmd.Args == nil {
		t.Fatal("Args should be set (MaximumNArgs(1))")
	}
	// 0 args OK
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("0 args should be valid: %v", err)
	}
	// 1 arg OK
	if err := cmd.Args(cmd, []string{"myapp"}); err != nil {
		t.Errorf("1 arg should be valid: %v", err)
	}
	// 2 args should fail
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("2 args should fail validation")
	}
}

func TestNewDownCmd(t *testing.T) {
	cmd := newDownCmd()
	if cmd.Use != "down [workload]" {
		t.Errorf("Use = %q, want 'down [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewDownCmd_AcceptsMaxOneArg(t *testing.T) {
	cmd := newDownCmd()
	if cmd.Args == nil {
		t.Fatal("Args should be set")
	}
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("0 args should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"myapp"}); err != nil {
		t.Errorf("1 arg should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("2 args should fail validation")
	}
}

func TestNewOpenCmd(t *testing.T) {
	cmd := newOpenCmd()
	if cmd.Use != "open [workload]" {
		t.Errorf("Use = %q, want 'open [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewOpenCmd_AcceptsMaxOneArg(t *testing.T) {
	cmd := newOpenCmd()
	if cmd.Args == nil {
		t.Fatal("Args should be set")
	}
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("0 args should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"myapp"}); err != nil {
		t.Errorf("1 arg should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("2 args should fail validation")
	}
}

func TestNewLogsCmd(t *testing.T) {
	cmd := newLogsCmd()
	if cmd.Use != "logs [workload]" {
		t.Errorf("Use = %q, want 'logs [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewLogsCmd_Flags(t *testing.T) {
	cmd := newLogsCmd()

	follow := cmd.Flags().Lookup("follow")
	if follow == nil {
		t.Fatal("missing --follow flag")
	}
	if follow.Shorthand != "f" {
		t.Errorf("follow shorthand = %q, want 'f'", follow.Shorthand)
	}
	if follow.DefValue != "false" {
		t.Errorf("follow default = %q, want 'false'", follow.DefValue)
	}

	tail := cmd.Flags().Lookup("tail")
	if tail == nil {
		t.Fatal("missing --tail flag")
	}
	if tail.Shorthand != "t" {
		t.Errorf("tail shorthand = %q, want 't'", tail.Shorthand)
	}
	if tail.DefValue != "100" {
		t.Errorf("tail default = %q, want '100'", tail.DefValue)
	}

	container := cmd.Flags().Lookup("container")
	if container == nil {
		t.Fatal("missing --container flag")
	}
	if container.Shorthand != "c" {
		t.Errorf("container shorthand = %q, want 'c'", container.Shorthand)
	}
	if container.DefValue != "" {
		t.Errorf("container default = %q, want empty", container.DefValue)
	}
}

func TestNewLogsCmd_AcceptsMaxOneArg(t *testing.T) {
	cmd := newLogsCmd()
	if cmd.Args == nil {
		t.Fatal("Args should be set")
	}
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("0 args should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"myapp"}); err != nil {
		t.Errorf("1 arg should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("2 args should fail validation")
	}
}

// --- Root/commands factory tests ---

func TestNewVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want 'version'", cmd.Use)
	}
	if cmd.Run == nil {
		t.Error("Run should be set")
	}
}

func TestNewInitCmd(t *testing.T) {
	cmd := newInitCmd()
	if cmd.Use != "init" {
		t.Errorf("Use = %q, want 'init'", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewStatusCmd(t *testing.T) {
	cmd := newStatusCmd()
	if cmd.Use != "status" {
		t.Errorf("Use = %q, want 'status'", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
	// Watch flag
	watch := cmd.Flags().Lookup("watch")
	if watch == nil {
		t.Fatal("missing --watch flag")
	}
	if watch.Shorthand != "w" {
		t.Errorf("watch shorthand = %q, want 'w'", watch.Shorthand)
	}
	// Interval flag
	interval := cmd.Flags().Lookup("interval")
	if interval == nil {
		t.Fatal("missing --interval flag")
	}
	if interval.DefValue != (10 * time.Second).String() {
		t.Errorf("interval default = %q, want %q", interval.DefValue, (10 * time.Second).String())
	}
}

func TestNewDiagnoseCmd(t *testing.T) {
	cmd := newDiagnoseCmd()
	if cmd.Use != "diagnose [resource]" {
		t.Errorf("Use = %q, want 'diagnose [resource]'", cmd.Use)
	}
	if cmd.Args == nil {
		t.Fatal("Args should require exactly 1")
	}
	// 0 args should fail
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("diagnose with 0 args should fail")
	}
	// 1 arg OK
	if err := cmd.Args(cmd, []string{"test"}); err != nil {
		t.Errorf("diagnose with 1 arg should pass: %v", err)
	}
	// 2 args should fail
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("diagnose with 2 args should fail")
	}
	// Bundle flag
	bundle := cmd.Flags().Lookup("bundle")
	if bundle == nil {
		t.Fatal("missing --bundle flag")
	}
}

func TestNewReconcileCmd(t *testing.T) {
	cmd := newReconcileCmd()
	if cmd.Use != "reconcile [resource]" {
		t.Errorf("Use = %q, want 'reconcile [resource]'", cmd.Use)
	}
	if cmd.Args == nil {
		t.Fatal("Args should require exactly 1")
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("reconcile with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"test"}); err != nil {
		t.Errorf("reconcile with 1 arg should pass: %v", err)
	}
}

func TestNewContextCmd(t *testing.T) {
	cmd := newContextCmd()
	if cmd.Use != "context" {
		t.Errorf("Use = %q, want 'context'", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestNewCompletionCmd(t *testing.T) {
	cmd := newCompletionCmd()
	if cmd.Use != "completion [bash|zsh|fish]" {
		t.Errorf("Use = %q, want 'completion [bash|zsh|fish]'", cmd.Use)
	}
	if cmd.Args == nil {
		t.Fatal("Args should require exactly 1")
	}
	// Valid args
	if len(cmd.ValidArgs) != 3 {
		t.Errorf("ValidArgs len = %d, want 3", len(cmd.ValidArgs))
	}
}

func TestNewTraceCmd(t *testing.T) {
	cmd := newTraceCmd()
	if cmd.Use != "trace [resource]" {
		t.Errorf("Use = %q, want 'trace [resource]'", cmd.Use)
	}
	if cmd.Args == nil {
		t.Fatal("Args should require exactly 1")
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("trace with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"test"}); err != nil {
		t.Errorf("trace with 1 arg should pass: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("trace with 2 args should fail")
	}
}

func TestNewAlertsCmd_Structure(t *testing.T) {
	cmd := newAlertsCmd()
	if cmd.Use != "alerts" {
		t.Errorf("Use = %q, want 'alerts'", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("missing --all flag")
	}
	if allFlag.DefValue != "false" {
		t.Errorf("all default = %q, want 'false'", allFlag.DefValue)
	}
}
