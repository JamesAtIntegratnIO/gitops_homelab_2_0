package deploy

import (
	"errors"
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
)

func TestNewDeployRemoveCmd_Structure(t *testing.T) {
	cmd := newDeployRemoveCmd()
	if cmd.Use != "remove [workload]" {
		t.Errorf("Use = %q, want 'remove [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}

	// Requires exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("remove should require exactly 1 arg")
	}
	if err := cmd.Args(cmd, []string{"my-workload"}); err != nil {
		t.Errorf("remove with 1 arg should pass: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("remove should reject 2 args")
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

func TestNewDeployRemoveCmd_RunEIsSet(t *testing.T) {
	cmd := newDeployRemoveCmd()
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestDeployRemove_FailsWithoutRepoPath(t *testing.T) {
	// Ensure config has no repo path and non-interactive mode
	config.Set(&config.Config{})
	defer config.Set(config.Default())

	cmd := newDeployRemoveCmd()
	err := cmd.RunE(cmd, []string{"some-workload"})
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
