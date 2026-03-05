package vcluster

import (
	"testing"
)

func TestIsCRDApp_Matches(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"cert-manager-crd", true},
		{"gateway-api-crds", true},
		{"gateway-api", true},
		{"my-app-crd-installer", true},
		{"CRD-UPPERCASE", true},
		{"Gateway-API-Resources", true},
	}
	for _, tt := range tests {
		got := isCRDApp(tt.name)
		if got != tt.want {
			t.Errorf("isCRDApp(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsCRDApp_NoMatch(t *testing.T) {
	tests := []string{
		"cert-manager",
		"nginx-ingress",
		"my-application",
		"prometheus",
		"external-secrets",
		"",
	}
	for _, name := range tests {
		if isCRDApp(name) {
			t.Errorf("isCRDApp(%q) = true, want false", name)
		}
	}
}

func TestSyncCmd_Structure(t *testing.T) {
	cmd := newSyncCmd()

	if cmd.Use != "sync [name]" {
		t.Errorf("Use = %q, want 'sync [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}

	// Requires exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("sync with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("sync with 1 arg should pass: %v", err)
	}
}

func TestSyncCmd_HasFlags(t *testing.T) {
	cmd := newSyncCmd()

	appFlag := cmd.Flags().Lookup("app")
	if appFlag == nil {
		t.Fatal("expected --app flag")
	}

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("expected --force flag")
	}
	if forceFlag.DefValue != "false" {
		t.Errorf("force default = %q, want 'false'", forceFlag.DefValue)
	}
}
