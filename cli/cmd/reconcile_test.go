package cmd

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

// runReconcile uses kube.SharedWithConfig directly (not the KubeClient interface),
// so we can't inject a fake. Instead we test the ancillary logic.

func TestVClusterOrchestratorV2GVR_IsSet(t *testing.T) {
	gvr := kube.VClusterOrchestratorV2GVR
	if gvr.Group == "" {
		t.Error("GVR Group should not be empty")
	}
	if gvr.Version == "" {
		t.Error("GVR Version should not be empty")
	}
	if gvr.Resource == "" {
		t.Error("GVR Resource should not be empty")
	}
}

func TestReconcileCmd_ArgsValidation(t *testing.T) {
	cmd := newReconcileCmd()

	// Should fail with 0 args
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("reconcile should require exactly 1 arg")
	}

	// Should succeed with 1 arg
	if err := cmd.Args(cmd, []string{"my-vcluster"}); err != nil {
		t.Errorf("reconcile with 1 arg should pass: %v", err)
	}

	// Should fail with 2 args
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("reconcile should reject 2 args")
	}
}

func TestReconcileCmd_HasRunE(t *testing.T) {
	cmd := newReconcileCmd()
	if cmd.RunE == nil {
		t.Error("reconcile RunE should be set")
	}
}
