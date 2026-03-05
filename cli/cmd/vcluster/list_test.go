package vcluster

import (
	"testing"
	"time"
)

func TestFormatAge_Minutes(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{0 * time.Minute, "0m"},
		{5 * time.Minute, "5m"},
		{30 * time.Minute, "30m"},
		{59 * time.Minute, "59m"},
	}
	for _, tt := range tests {
		got := formatAge(tt.duration)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}

func TestFormatAge_Hours(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{1 * time.Hour, "1h"},
		{6 * time.Hour, "6h"},
		{23 * time.Hour, "23h"},
	}
	for _, tt := range tests {
		got := formatAge(tt.duration)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}

func TestFormatAge_Days(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{7 * 24 * time.Hour, "7d"},
	}
	for _, tt := range tests {
		got := formatAge(tt.duration)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}

func TestListCmd_Structure(t *testing.T) {
	cmd := newListCmd()

	if cmd.Use != "list" {
		t.Errorf("Use = %q, want 'list'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Check alias
	found := false
	for _, a := range cmd.Aliases {
		if a == "ls" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected alias 'ls', got %v", cmd.Aliases)
	}

	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}
