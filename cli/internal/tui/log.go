package tui

import (
	"fmt"
	"os"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

// Log provides leveled output helpers that respect --verbose, --quiet, and
// structured output modes. Use these instead of raw fmt.Print in commands.

// Debug prints a message only when --verbose is set.
// Suppressed in structured (JSON/YAML) output mode.
func Debug(format string, args ...any) {
	if IsStructured() {
		return
	}
	cfg := config.Get()
	if !cfg.Verbose {
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
	cfg := config.Get()
	if cfg.Quiet {
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
	cfg := config.Get()
	if cfg.Quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Println(SuccessStyle.Render(IconCheck) + " " + msg)
}
