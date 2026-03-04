package vcluster

import (
	"testing"
)

func TestParseEgressRule_Valid(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantCIDR string
		wantPort int
		wantProto string
	}{
		{"postgres:10.0.1.50/32:5432", "postgres", "10.0.1.50/32", 5432, "TCP"},
		{"redis:10.0.1.60/32:6379:TCP", "redis", "10.0.1.60/32", 6379, "TCP"},
		{"dns:10.0.0.1/32:53:UDP", "dns", "10.0.0.1/32", 53, "UDP"},
		{"mysql:192.168.1.0/24:3306:tcp", "mysql", "192.168.1.0/24", 3306, "TCP"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rule, err := parseEgressRule(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rule.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", rule.Name, tt.wantName)
			}
			if rule.CIDR != tt.wantCIDR {
				t.Errorf("CIDR = %q, want %q", rule.CIDR, tt.wantCIDR)
			}
			if rule.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", rule.Port, tt.wantPort)
			}
			if rule.Protocol != tt.wantProto {
				t.Errorf("Protocol = %q, want %q", rule.Protocol, tt.wantProto)
			}
		})
	}
}

func TestParseEgressRule_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too few parts", "postgres:10.0.1.50/32"},
		{"missing port", "name:cidr"},
		{"bad port", "pg:10.0.0.1/32:abc"},
		{"bad protocol", "pg:10.0.0.1/32:5432:ICMP"},
		{"single part", "just-a-name"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEgressRule(tt.input)
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

func TestParseKeyValue_Valid(t *testing.T) {
	tests := []struct {
		input    string
		wantKey  string
		wantVal  string
	}{
		{"env=production", "env", "production"},
		{"key=value=with=equals", "key", "value=with=equals"},
		{"flag=", "flag", ""},
		{"name=my-cluster", "name", "my-cluster"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			k, v, err := parseKeyValue(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k != tt.wantKey {
				t.Errorf("key = %q, want %q", k, tt.wantKey)
			}
			if v != tt.wantVal {
				t.Errorf("value = %q, want %q", v, tt.wantVal)
			}
		})
	}
}

func TestParseKeyValue_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no equals", "justkey"},
		{"empty key", "=value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseKeyValue(tt.input)
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "vcluster" {
		t.Errorf("expected Use 'vcluster', got %q", cmd.Use)
	}

	// Verify subcommands are registered
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	for _, expected := range []string{"create", "delete", "list", "status", "kubeconfig", "sync", "apps"} {
		if !subNames[expected] {
			t.Errorf("missing subcommand %q", expected)
		}
	}
}
