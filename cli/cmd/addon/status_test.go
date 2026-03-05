package addon

import "testing"

func TestStatusCmd_RejectsMultipleArgs(t *testing.T) {
	cmd := newAddonStatusCmd()

	// 3 args should fail
	if err := cmd.Args(cmd, []string{"a", "b", "c"}); err == nil {
		t.Error("status should reject 3 args")
	}

	// 1 arg should pass
	if err := cmd.Args(cmd, []string{"my-addon"}); err != nil {
		t.Errorf("status should accept 1 arg: %v", err)
	}
}

func TestStatusCmd_LongDescription(t *testing.T) {
	cmd := newAddonStatusCmd()

	// The Args field on status requires ExactArgs(1)
	if cmd.Args == nil {
		t.Error("Args validator should be set")
	}
	if cmd.Use == "" {
		t.Error("Use should not be empty")
	}
}
