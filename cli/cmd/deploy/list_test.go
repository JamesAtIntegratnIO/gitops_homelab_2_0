package deploy

import (
	"testing"
)

func TestNewDeployListCmd_Structure(t *testing.T) {
	cmd := newDeployListCmd()
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want 'list'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Check alias
	foundAlias := false
	for _, a := range cmd.Aliases {
		if a == "ls" {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Error("list should have 'ls' alias")
	}

	// Verify --cluster flag
	clusterFlag := cmd.Flags().Lookup("cluster")
	if clusterFlag == nil {
		t.Fatal("missing --cluster flag")
	}
	if clusterFlag.DefValue != "" {
		t.Errorf("--cluster default = %q, want empty", clusterFlag.DefValue)
	}
}

func TestNewDeployListCmd_RunEIsSet(t *testing.T) {
	cmd := newDeployListCmd()
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}
