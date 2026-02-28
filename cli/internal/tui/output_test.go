package tui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSetOutputFormat(t *testing.T) {
	tests := []struct {
		input string
		want  OutputFormat
	}{
		{"json", FormatJSON},
		{"yaml", FormatYAML},
		{"text", FormatText},
		{"", FormatText},
		{"unknown", FormatText},
	}
	for _, tt := range tests {
		SetOutputFormat(tt.input)
		got := GetOutputFormat()
		if got != tt.want {
			t.Errorf("SetOutputFormat(%q) → GetOutputFormat() = %q, want %q", tt.input, got, tt.want)
		}
	}
	// Restore default
	SetOutputFormat("text")
}

func TestIsStructured(t *testing.T) {
	SetOutputFormat("json")
	if !IsStructured() {
		t.Error("expected IsStructured=true for json")
	}
	SetOutputFormat("yaml")
	if !IsStructured() {
		t.Error("expected IsStructured=true for yaml")
	}
	SetOutputFormat("text")
	if IsStructured() {
		t.Error("expected IsStructured=false for text")
	}
}

func TestFprintOutputJSON(t *testing.T) {
	SetOutputFormat("json")
	defer SetOutputFormat("text")

	var buf bytes.Buffer
	data := map[string]string{"name": "test", "status": "ok"}
	err := FprintOutput(&buf, data, "this is text")
	if err != nil {
		t.Fatalf("FprintOutput error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nGot: %s", err, buf.String())
	}
	if result["name"] != "test" || result["status"] != "ok" {
		t.Errorf("unexpected JSON values: %+v", result)
	}
}

func TestFprintOutputYAML(t *testing.T) {
	SetOutputFormat("yaml")
	defer SetOutputFormat("text")

	var buf bytes.Buffer
	data := map[string]string{"name": "test"}
	err := FprintOutput(&buf, data, "this is text")
	if err != nil {
		t.Fatalf("FprintOutput error: %v", err)
	}

	var result map[string]string
	if err := yaml.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid YAML output: %v\nGot: %s", err, buf.String())
	}
	if result["name"] != "test" {
		t.Errorf("unexpected YAML values: %+v", result)
	}
}

func TestFprintOutputText(t *testing.T) {
	SetOutputFormat("text")

	var buf bytes.Buffer
	data := map[string]string{"name": "test"}
	err := FprintOutput(&buf, data, "plain text output\n")
	if err != nil {
		t.Fatalf("FprintOutput error: %v", err)
	}

	got := buf.String()
	if got != "plain text output\n" {
		t.Errorf("expected text output, got %q", got)
	}
}

func TestPrintStructuredReturnsCorrectly(t *testing.T) {
	SetOutputFormat("text")
	if PrintStructured(map[string]string{"a": "b"}) {
		t.Error("PrintStructured should return false in text mode")
	}

	// In JSON mode it should return true (output goes to stdout, can't easily capture)
	SetOutputFormat("json")
	defer SetOutputFormat("text")
	if !PrintStructured(map[string]string{"a": "b"}) {
		t.Error("PrintStructured should return true in json mode")
	}
}

func TestRenderOutputIntegration(t *testing.T) {
	// Smoke test: RenderOutput writes to stdout — we just verify no panic/error
	SetOutputFormat("text")
	err := RenderOutput(nil, "")
	if err != nil {
		t.Errorf("RenderOutput text mode error: %v", err)
	}
}

func TestFprintOutputJSONPrettyPrint(t *testing.T) {
	SetOutputFormat("json")
	defer SetOutputFormat("text")

	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	_ = FprintOutput(&buf, data, "")

	// Should be indented (pretty-printed)
	if !strings.Contains(buf.String(), "  ") {
		t.Error("JSON output should be pretty-printed with indentation")
	}
}
