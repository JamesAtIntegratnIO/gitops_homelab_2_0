package scale

import (
	"testing"
)

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "scale" {
		t.Errorf("expected Use 'scale', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Verify subcommands
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	for _, expected := range []string{"down", "up"} {
		if !subNames[expected] {
			t.Errorf("missing subcommand %q", expected)
		}
	}
}

func TestScaleDownCmd_RequiresArg(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "down" {
			// ExactArgs(1) means 0 args should fail validation
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error when no args provided to 'down'")
			}
			// 1 arg should pass
			if err := sub.Args(sub, []string{"test-ns"}); err != nil {
				t.Errorf("unexpected error with 1 arg: %v", err)
			}
			// 2 args should fail
			if err := sub.Args(sub, []string{"a", "b"}); err == nil {
				t.Error("expected error when 2 args provided to 'down'")
			}
			return
		}
	}
	t.Fatal("down subcommand not found")
}

func TestScaleUpCmd_RequiresArg(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "up" {
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error when no args provided to 'up'")
			}
			if err := sub.Args(sub, []string{"test-ns"}); err != nil {
				t.Errorf("unexpected error with 1 arg: %v", err)
			}
			if err := sub.Args(sub, []string{"a", "b"}); err == nil {
				t.Error("expected error when 2 args provided to 'up'")
			}
			return
		}
	}
	t.Fatal("up subcommand not found")
}

func TestScaleDownCmd_HasDescription(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "down" {
			if sub.Short == "" {
				t.Error("down: expected non-empty Short description")
			}
			if sub.Long == "" {
				t.Error("down: expected non-empty Long description")
			}
			return
		}
	}
}

func TestScaleUpCmd_HasDescription(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "up" {
			if sub.Short == "" {
				t.Error("up: expected non-empty Short description")
			}
			return
		}
	}
}
