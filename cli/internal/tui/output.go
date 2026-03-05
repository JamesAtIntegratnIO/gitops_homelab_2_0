package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// OutputFormat controls how structured data is rendered.
type OutputFormat string

const (
	// FormatText renders human-friendly TUI output (default).
	FormatText OutputFormat = "text"
	// FormatJSON renders JSON output.
	FormatJSON OutputFormat = "json"
	// FormatYAML renders YAML output.
	FormatYAML OutputFormat = "yaml"
)

// outputFormat is the currently configured output format.
var outputFormat OutputFormat = FormatText

// SetOutputFormat sets the global output format.
func SetOutputFormat(format string) {
	switch format {
	case "json":
		outputFormat = FormatJSON
	case "yaml":
		outputFormat = FormatYAML
	default:
		outputFormat = FormatText
	}
}

// GetOutputFormat returns the active output format.
func GetOutputFormat() OutputFormat {
	return outputFormat
}

// IsStructured returns true when output should be machine-readable (JSON or YAML).
func IsStructured() bool {
	return outputFormat == FormatJSON || outputFormat == FormatYAML
}

// RenderOutput prints data in the configured output format.
// For text format, it prints the textOutput string as-is.
// For JSON/YAML, it marshals the data parameter.
func RenderOutput(data any, textOutput string) error {
	return FprintOutput(os.Stdout, data, textOutput)
}

// FprintOutput writes structured or text output to a writer.
func FprintOutput(w io.Writer, data any, textOutput string) error {
	switch outputFormat {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(data)
	default:
		_, err := fmt.Fprint(w, textOutput)
		return err
	}
}

// PrintStructured prints data as JSON or YAML depending on the output format.
// Returns true if it printed (i.e., structured mode was active), false if text mode.
// An error is returned if encoding fails (e.g., unmarshalable data).
// This is useful for commands that want to short-circuit TUI rendering.
func PrintStructured(data any) (bool, error) {
	if !IsStructured() {
		return false, nil
	}
	switch outputFormat {
	case FormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return true, fmt.Errorf("encoding JSON: %w", err)
		}
	case FormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(data); err != nil {
			enc.Close()
			return true, fmt.Errorf("encoding YAML: %w", err)
		}
		enc.Close()
	}
	return true, nil
}
