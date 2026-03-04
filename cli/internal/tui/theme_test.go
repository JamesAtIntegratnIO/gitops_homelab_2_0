package tui

import (
	"strings"
	"testing"
)

func TestStatusIcon(t *testing.T) {
	ok := StatusIcon(true)
	if !strings.Contains(ok, IconCheck) {
		t.Errorf("StatusIcon(true) should contain %q, got %q", IconCheck, ok)
	}
	fail := StatusIcon(false)
	if !strings.Contains(fail, IconCross) {
		t.Errorf("StatusIcon(false) should contain %q, got %q", IconCross, fail)
	}
}

func TestDiagIcon(t *testing.T) {
	tests := []struct {
		level    int
		contains string
	}{
		{0, IconCheck},
		{1, IconWarn},
		{2, IconCross},
		{99, "?"},
		{-1, "?"},
	}
	for _, tt := range tests {
		got := DiagIcon(tt.level)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("DiagIcon(%d) should contain %q, got %q", tt.level, tt.contains, got)
		}
	}
}

func TestSeverityBadge(t *testing.T) {
	tests := []struct {
		severity string
		contains string
	}{
		{"critical", "CRIT"},
		{"warning", "WARN"},
		{"info", "INFO"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, tt := range tests {
		got := SeverityBadge(tt.severity)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("SeverityBadge(%q) should contain %q, got %q", tt.severity, tt.contains, got)
		}
	}
}

func TestSyncBadge(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"Synced", "Synced"},
		{"OutOfSync", "OutOfSync"},
		{"Unknown", "Unknown"},
		{"Other", "Other"},
	}
	for _, tt := range tests {
		got := SyncBadge(tt.status)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("SyncBadge(%q) should contain %q, got %q", tt.status, tt.contains, got)
		}
	}
}

func TestHealthBadge(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"Healthy", "Healthy"},
		{"Degraded", "Degraded"},
		{"Progressing", "Progressing"},
		{"Missing", "Missing"},
		{"Suspended", "Suspended"},
		{"Other", "Other"},
	}
	for _, tt := range tests {
		got := HealthBadge(tt.status)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("HealthBadge(%q) should contain %q, got %q", tt.status, tt.contains, got)
		}
	}
}

func TestOpBadge(t *testing.T) {
	// Empty phase
	got := OpBadge("", 0)
	if !strings.Contains(got, "—") {
		t.Errorf("OpBadge('', 0) should contain '—', got %q", got)
	}

	// Succeeded
	got = OpBadge("Succeeded", 0)
	if !strings.Contains(got, "Succeeded") {
		t.Errorf("OpBadge('Succeeded', 0) should contain 'Succeeded', got %q", got)
	}

	// Running
	got = OpBadge("Running", 0)
	if !strings.Contains(got, "Running") {
		t.Errorf("OpBadge('Running', 0) should contain 'Running', got %q", got)
	}

	// Failed
	got = OpBadge("Failed", 0)
	if !strings.Contains(got, "Failed") {
		t.Errorf("OpBadge('Failed', 0) should contain 'Failed', got %q", got)
	}

	// With retry count
	got = OpBadge("Succeeded", 3)
	if !strings.Contains(got, "retry:3") {
		t.Errorf("OpBadge('Succeeded', 3) should contain 'retry:3', got %q", got)
	}

	// Error phase
	got = OpBadge("Error", 0)
	if !strings.Contains(got, "Error") {
		t.Errorf("OpBadge('Error', 0) should contain 'Error', got %q", got)
	}

	// Unknown phase
	got = OpBadge("SomeOther", 0)
	if !strings.Contains(got, "SomeOther") {
		t.Errorf("OpBadge('SomeOther', 0) should contain 'SomeOther', got %q", got)
	}
}

func TestKeyValue(t *testing.T) {
	got := KeyValue("Phase", "Ready")
	if !strings.Contains(got, "Phase") {
		t.Errorf("KeyValue should contain the key 'Phase', got %q", got)
	}
	if !strings.Contains(got, "Ready") {
		t.Errorf("KeyValue should contain the value 'Ready', got %q", got)
	}
}

func TestSectionHeader(t *testing.T) {
	got := SectionHeader("Nodes")
	if !strings.Contains(got, "Nodes") {
		t.Errorf("SectionHeader should contain 'Nodes', got %q", got)
	}
	// Should start with newline
	if !strings.HasPrefix(got, "\n") {
		t.Errorf("SectionHeader should start with newline, got %q", got)
	}
}

func TestBox(t *testing.T) {
	got := Box("content here")
	if !strings.Contains(got, "content here") {
		t.Errorf("Box should contain 'content here', got %q", got)
	}
	// Should contain border characters
	if !strings.Contains(got, "╭") && !strings.Contains(got, "─") {
		t.Errorf("Box should contain border characters, got %q", got)
	}
}

func TestDivider(t *testing.T) {
	got := Divider(10)
	if !strings.Contains(got, "──────────") {
		t.Errorf("Divider(10) should contain 10 dashes, got %q", got)
	}

	// Default width when <=0
	got = Divider(0)
	if got == "" {
		t.Error("Divider(0) should return a non-empty string")
	}

	got = Divider(-5)
	if got == "" {
		t.Error("Divider(-5) should return a non-empty string")
	}
}

func TestIndent(t *testing.T) {
	// Single level indent
	got := Indent("line1\nline2", 1)
	lines := strings.Split(got, "\n")
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "  ") {
			t.Errorf("Indent level 1 should prefix with 2 spaces, got %q", l)
		}
	}

	// Double level indent
	got = Indent("line1\nline2", 2)
	lines = strings.Split(got, "\n")
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "    ") {
			t.Errorf("Indent level 2 should prefix with 4 spaces, got %q", l)
		}
	}

	// Empty lines should not be padded
	got = Indent("line1\n\nline3", 1)
	lines = strings.Split(got, "\n")
	if lines[1] != "" {
		t.Errorf("empty lines should remain empty, got %q", lines[1])
	}

	// Zero indent does nothing
	got = Indent("line1", 0)
	if got != "line1" {
		t.Errorf("Indent level 0 should not change, got %q", got)
	}
}

func TestValueOrMuted(t *testing.T) {
	// Non-empty value
	got := ValueOrMuted("hello", "placeholder")
	if got != "hello" {
		t.Errorf("ValueOrMuted with value should return value, got %q", got)
	}

	// Empty value returns muted placeholder
	got = ValueOrMuted("", "placeholder")
	if !strings.Contains(got, "placeholder") {
		t.Errorf("ValueOrMuted with empty value should contain placeholder, got %q", got)
	}
}

func TestStyledCount(t *testing.T) {
	// Zero value is not styled
	got := StyledCount(0, ErrorStyle)
	if got != "0" {
		t.Errorf("StyledCount(0) should return '0', got %q", got)
	}

	// Non-zero positive value should contain the number (styling depends on TTY)
	got = StyledCount(5, ErrorStyle)
	if !strings.Contains(got, "5") {
		t.Errorf("StyledCount(5) should contain '5', got %q", got)
	}

	// Negative value: n > 0 is false, so it should return plain text
	got = StyledCount(-1, WarningStyle)
	if got != "-1" {
		t.Errorf("StyledCount(-1) should return plain '-1' (not > 0), got %q", got)
	}
}

func TestSyncBadge_ContainsIcons(t *testing.T) {
	synced := SyncBadge("Synced")
	if !strings.Contains(synced, IconCheck) {
		t.Errorf("SyncBadge('Synced') should contain check icon, got %q", synced)
	}

	oos := SyncBadge("OutOfSync")
	if !strings.Contains(oos, IconSync) {
		t.Errorf("SyncBadge('OutOfSync') should contain sync icon, got %q", oos)
	}
}

func TestHealthBadge_ContainsIcons(t *testing.T) {
	healthy := HealthBadge("Healthy")
	if !strings.Contains(healthy, IconHeart) {
		t.Errorf("HealthBadge('Healthy') should contain heart icon, got %q", healthy)
	}

	degraded := HealthBadge("Degraded")
	if !strings.Contains(degraded, IconCross) {
		t.Errorf("HealthBadge('Degraded') should contain cross icon, got %q", degraded)
	}

	progressing := HealthBadge("Progressing")
	if !strings.Contains(progressing, IconSync) {
		t.Errorf("HealthBadge('Progressing') should contain sync icon, got %q", progressing)
	}

	suspended := HealthBadge("Suspended")
	if !strings.Contains(suspended, IconPause) {
		t.Errorf("HealthBadge('Suspended') should contain pause icon, got %q", suspended)
	}
}
