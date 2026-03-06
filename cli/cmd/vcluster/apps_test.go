package vcluster

import (
	"testing"
	"time"
)

func TestIsStaleOperation_EmptyString(t *testing.T) {
	if isStaleOperation("") {
		t.Error("empty string should not be stale")
	}
}

func TestIsStaleOperation_InvalidTimestamp(t *testing.T) {
	if isStaleOperation("not-a-date") {
		t.Error("invalid timestamp should not be stale")
	}
}

func TestIsStaleOperation_RecentNotStale(t *testing.T) {
	recent := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	if isStaleOperation(recent) {
		t.Error("operation started 10 minutes ago should not be stale")
	}
}

func TestIsStaleOperation_OldIsStale(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	if !isStaleOperation(old) {
		t.Error("operation started 2 hours ago should be stale")
	}
}

func TestIsStaleOperation_ExactlyOneHourNotStale(t *testing.T) {
	// Exactly 1 hour ago — isStaleOperation requires > 1 hour
	// Use a small buffer for test timing
	almost := time.Now().Add(-59 * time.Minute).Format(time.RFC3339)
	if isStaleOperation(almost) {
		t.Error("operation started 59 minutes ago should not be stale")
	}
}

func TestTruncateMessage_ShortMessage(t *testing.T) {
	msg := "short message"
	got := truncateMessage(msg, 100)
	if got != msg {
		t.Errorf("truncateMessage(%q, 100) = %q, want %q", msg, got, msg)
	}
}

func TestTruncateMessage_ExactLength(t *testing.T) {
	msg := "hello"
	got := truncateMessage(msg, 5)
	if got != msg {
		t.Errorf("truncateMessage(%q, 5) = %q, want %q", msg, got, msg)
	}
}

func TestTruncateMessage_LongMessage(t *testing.T) {
	msg := "this is a long error message that should be truncated"
	got := truncateMessage(msg, 20)
	if len(got) != 23 { // 20 + "..."
		t.Errorf("truncateMessage length = %d, want 23", len(got))
	}
	if got[len(got)-3:] != "..." {
		t.Error("truncated message should end with '...'")
	}
}

func TestTruncateMessage_EmptyMessage(t *testing.T) {
	got := truncateMessage("", 10)
	if got != "" {
		t.Errorf("empty message should remain empty, got %q", got)
	}
}

func TestAppsCmd_Structure(t *testing.T) {
	cmd := newAppsCmd()

	if cmd.Use != "apps [name]" {
		t.Errorf("Use = %q, want 'apps [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}

	// Should require exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("apps should require exactly 1 arg")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("apps with 1 arg should pass: %v", err)
	}
}
