package tui

import (
	"fmt"
	"os"
)

// verbose and quiet are package-level flags set at initialisation time via
// SetVerbose / SetQuiet. This avoids a reverse dependency on config.
var (
	verbose bool
	quiet   bool
)

// SetVerbose enables or disables verbose (debug) output.
func SetVerbose(v bool) { verbose = v }

// SetQuiet enables or disables quiet mode (suppresses info/success output).
func SetQuiet(q bool) { quiet = q }

// Log provides leveled output helpers that respect --verbose, --quiet, and
// structured output modes. Use these instead of raw fmt.Print in commands.

// Debug prints a message only when --verbose is set.
// Suppressed in structured (JSON/YAML) output mode.
func Debug(format string, args ...any) {
	if IsStructured() {
		return
	}
	if !verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, DimStyle.Render("DEBUG: "+msg))
}

// Info prints informational messages (the default).
// Suppressed by --quiet and structured output modes.
func Info(format string, args ...any) {
	if IsStructured() {
		return
	}
	if quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Warn prints a warning message. Only suppressed in structured mode.
func Warn(format string, args ...any) {
	if IsStructured() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, WarningStyle.Render(IconWarn+" "+msg))
}

// Error prints an error message. Never suppressed (goes to stderr).
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if IsStructured() {
		fmt.Fprintln(os.Stderr, "error: "+msg)
		return
	}
	fmt.Fprintln(os.Stderr, ErrorStyle.Render(IconCross+" "+msg))
}

// Success prints a success indicator. Suppressed by --quiet and structured output.
func Success(format string, args ...any) {
	if IsStructured() {
		return
	}
	if quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Println(SuccessStyle.Render(IconCheck) + " " + msg)
}
