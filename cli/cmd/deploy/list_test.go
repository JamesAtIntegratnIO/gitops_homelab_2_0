package deploy

import (
	"errors"
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
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

func TestDeployList_FailsWithoutRepoPath(t *testing.T) {
	// Ensure config has no repo path set
	config.Set(&config.Config{})
	defer config.Set(config.Default())

	cmd := newDeployListCmd()
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when RepoPath is empty")
	}

	var hErr *hcerrors.HctlError
	if !errors.As(err, &hErr) {
		t.Fatalf("expected HctlError, got %T: %v", err, err)
	}
	if hErr.Code != hcerrors.ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", hErr.Code, hcerrors.ExitUserError)
	}
	if !strings.Contains(err.Error(), "repo path") {
		t.Errorf("error should mention repo path, got: %v", err)
	}
}
