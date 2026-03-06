package secret

import (
	"testing"
)

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "secret" {
		t.Errorf("expected Use 'secret', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Verify subcommands
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	for _, expected := range []string{"get", "list"} {
		if !subNames[expected] {
			t.Errorf("missing subcommand %q", expected)
		}
	}
}

func TestSecretGetCmd_RequiresArgs(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			// ExactArgs(2)
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error with 0 args")
			}
			if err := sub.Args(sub, []string{"ns"}); err == nil {
				t.Error("expected error with 1 arg")
			}
			if err := sub.Args(sub, []string{"ns", "name"}); err != nil {
				t.Errorf("unexpected error with 2 args: %v", err)
			}
			if err := sub.Args(sub, []string{"ns", "name", "extra"}); err == nil {
				t.Error("expected error with 3 args")
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

func TestSecretListCmd_RequiresArgs(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			// ExactArgs(1)
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error with 0 args")
			}
			if err := sub.Args(sub, []string{"ns"}); err != nil {
				t.Errorf("unexpected error with 1 arg: %v", err)
			}
			if err := sub.Args(sub, []string{"ns", "extra"}); err == nil {
				t.Error("expected error with 2 args")
			}
			return
		}
	}
	t.Fatal("list subcommand not found")
}

func TestSecretListCmd_HasAlias(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found := false
			for _, alias := range sub.Aliases {
				if alias == "ls" {
					found = true
					break
				}
			}
			if !found {
				t.Error("list command should have 'ls' alias")
			}
			return
		}
	}
}
