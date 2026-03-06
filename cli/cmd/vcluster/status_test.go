package vcluster

import (
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/platform"
)

func TestStatusCmd_Structure(t *testing.T) {
	cmd := newStatusCmd()

	if cmd.Use != "status [name]" {
		t.Errorf("Use = %q, want 'status [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}

	// Requires exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("status with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("status with 1 arg should pass: %v", err)
	}
}

func TestStatusCmd_HasDiagnoseFlag(t *testing.T) {
	cmd := newStatusCmd()

	f := cmd.Flags().Lookup("diagnose")
	if f == nil {
		t.Fatal("expected --diagnose flag")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestFormatStatusContract_BasicFields(t *testing.T) {
	sc := &platform.StatusContract{
		Phase:   "Ready",
		Message: "All components healthy",
	}

	output := FormatStatusContract("test-vc", sc)
	if !strings.Contains(output, "test-vc") {
		t.Error("output should contain cluster name")
	}
	if !strings.Contains(output, "Ready") {
		t.Error("output should contain phase")
	}
	if !strings.Contains(output, "All components healthy") {
		t.Error("output should contain message")
	}
}

func TestFormatStatusContract_WithEndpoints(t *testing.T) {
	sc := &platform.StatusContract{
		Phase: "Ready",
		Endpoints: platform.StatusEndpoints{
			API:    "https://test.cluster.example.com",
			ArgoCD: "https://argocd-test.cluster.example.com",
		},
	}

	output := FormatStatusContract("test-vc", sc)
	if !strings.Contains(output, "https://test.cluster.example.com") {
		t.Error("output should contain API endpoint")
	}
	if !strings.Contains(output, "https://argocd-test.cluster.example.com") {
		t.Error("output should contain ArgoCD endpoint")
	}
}

func TestFormatStatusContract_WithHealth(t *testing.T) {
	sc := &platform.StatusContract{
		Phase: "Ready",
		Health: platform.StatusHealth{
			ArgoCDSync:   "Synced",
			ArgoCDHealth: "Healthy",
			PodsReady:    5,
			PodsTotal:    5,
		},
	}

	output := FormatStatusContract("test-vc", sc)
	if !strings.Contains(output, "Synced") {
		t.Error("output should contain sync status")
	}
	if !strings.Contains(output, "5/5") {
		t.Error("output should contain pod count")
	}
}

func TestFormatStatusContract_WithConditions(t *testing.T) {
	sc := &platform.StatusContract{
		Phase: "Ready",
		Conditions: []platform.StatusCondition{
			{Type: "KratixResourceReady", Status: "True", Reason: "Reconciled"},
			{Type: "VClusterReady", Status: "False", Reason: "Pending"},
		},
	}

	output := FormatStatusContract("test-vc", sc)
	if !strings.Contains(output, "KratixResourceReady") {
		t.Error("output should contain condition type")
	}
	if !strings.Contains(output, "Reconciled") {
		t.Error("output should contain reason")
	}
	if !strings.Contains(output, "VClusterReady") {
		t.Error("output should contain second condition")
	}
}

func TestFormatTimeAgo_RecentTimestamp(t *testing.T) {
	// Empty timestamp
	got := formatTimeAgo("")
	if got != "unknown" {
		t.Errorf("formatTimeAgo(\"\") = %q, want \"unknown\"", got)
	}

	// Invalid timestamp
	got = formatTimeAgo("not-a-date")
	if got != "not-a-date" {
		t.Errorf("formatTimeAgo(\"not-a-date\") = %q, want \"not-a-date\"", got)
	}
}
