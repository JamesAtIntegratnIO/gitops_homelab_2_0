package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Color Palette
//
// Single source of truth for every color used across the CLI.
// Uses 256-color indices for rich rendering on modern terminals.
// ─────────────────────────────────────────────────────────────────────────

var (
	ColorAccent   = lipgloss.Color("99")  // Vibrant purple — brand accent
	ColorAccentBg = lipgloss.Color("56")  // Deep purple — banners, active tabs
	ColorGreen    = lipgloss.Color("78")  // Emerald — success, healthy, ready
	ColorYellow   = lipgloss.Color("220") // Amber — warnings, in-progress
	ColorRed      = lipgloss.Color("203") // Rose — errors, critical, failed
	ColorCyan     = lipgloss.Color("81")  // Cyan — info, commands, links
	ColorGray     = lipgloss.Color("245") // Gray — secondary/muted text
	ColorSubtle   = lipgloss.Color("238") // Dark gray — borders, separators
	ColorWhite    = lipgloss.Color("252") // Soft white — primary body text
	ColorBright   = lipgloss.Color("15")  // Pure white — headings, emphasis
	ColorOverlay  = lipgloss.Color("236") // Dark background for modals
)

// ─────────────────────────────────────────────────────────────────────────
// Icons
//
// Consistent unicode glyphs across the CLI.  No emoji — they have variable
// width and render inconsistently across terminals.
// ─────────────────────────────────────────────────────────────────────────

const (
	IconCheck   = "✓"
	IconCross   = "✗"
	IconWarn    = "▲"
	IconPending = "○"
	IconArrow   = "→"
	IconBullet  = "•"
	IconDot     = "·"
	IconPlay    = "▸"
	IconSync    = "⟳"
	IconHeart   = "♥"
	IconPause   = "⏸"
	IconBell    = "◆"
)

// ─────────────────────────────────────────────────────────────────────────
// Semantic Styles
//
// Use these throughout command code.  Never construct ad-hoc
// lipgloss.NewStyle() with raw color literals in command files.
// ─────────────────────────────────────────────────────────────────────────

var (
	// Text hierarchy
	TitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	HeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorBright)
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	// Semantic colors
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorGreen)
	WarningStyle = lipgloss.NewStyle().Foreground(ColorYellow)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ColorRed)
	InfoStyle    = lipgloss.NewStyle().Foreground(ColorCyan)
	MutedStyle   = lipgloss.NewStyle().Foreground(ColorGray)
	SubtleStyle  = lipgloss.NewStyle().Foreground(ColorSubtle)
	CodeStyle    = lipgloss.NewStyle().Foreground(ColorCyan)

	// Backward-compatibility aliases
	DimStyle      = lipgloss.NewStyle().Foreground(ColorGray)
	HeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(ColorBright).PaddingBottom(1)
	SelectedStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	// Layout components
	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBright).
			Background(ColorAccentBg).
			Padding(0, 2)

	boxBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccentBg).
			Padding(1, 2)

	keyLabel = lipgloss.NewStyle().Foreground(ColorGray).Width(16)
)

// ─────────────────────────────────────────────────────────────────────────
// Component Functions
//
// Reusable rendering primitives that keep output consistent across every
// command.  Import and call these instead of hand-formatting with fmt.
// ─────────────────────────────────────────────────────────────────────────

// StatusIcon returns a colored ✓ or ✗.
func StatusIcon(ok bool) string {
	if ok {
		return SuccessStyle.Render(IconCheck)
	}
	return ErrorStyle.Render(IconCross)
}

// DiagIcon returns a styled diagnostic status icon.
//
//	0 = OK (green ✓)   1 = Warning (yellow ▲)
//	2 = Error (red ✗)  anything else = unknown (gray ?)
func DiagIcon(level int) string {
	switch level {
	case 0:
		return SuccessStyle.Render(IconCheck)
	case 1:
		return WarningStyle.Render(IconWarn)
	case 2:
		return ErrorStyle.Render(IconCross)
	default:
		return MutedStyle.Render("?")
	}
}

// SeverityBadge returns a colored severity label for alerts.
func SeverityBadge(severity string) string {
	switch severity {
	case "critical":
		return ErrorStyle.Bold(true).Render("CRIT")
	case "warning":
		return WarningStyle.Bold(true).Render("WARN")
	case "info":
		return MutedStyle.Render("INFO")
	default:
		return MutedStyle.Render(severity)
	}
}

// SyncBadge formats an ArgoCD sync status with a leading icon.
func SyncBadge(status string) string {
	switch status {
	case "Synced":
		return SuccessStyle.Render(IconCheck + " Synced")
	case "OutOfSync":
		return WarningStyle.Render(IconSync + " OutOfSync")
	case "Unknown":
		return MutedStyle.Render("? Unknown")
	default:
		return MutedStyle.Render(status)
	}
}

// HealthBadge formats an ArgoCD health status with a leading icon.
func HealthBadge(status string) string {
	switch status {
	case "Healthy":
		return SuccessStyle.Render(IconHeart + " Healthy")
	case "Degraded":
		return ErrorStyle.Render(IconCross + " Degraded")
	case "Progressing":
		return WarningStyle.Render(IconSync + " Progressing")
	case "Missing":
		return ErrorStyle.Render(IconCross + " Missing")
	case "Suspended":
		return MutedStyle.Render(IconPause + " Suspended")
	default:
		return MutedStyle.Render(status)
	}
}

// OpBadge formats an ArgoCD operation phase.
func OpBadge(phase string, retryCount int64) string {
	if phase == "" {
		return MutedStyle.Render("—")
	}
	info := phase
	if retryCount > 0 {
		info += fmt.Sprintf(" retry:%d", retryCount)
	}
	switch phase {
	case "Succeeded":
		return SuccessStyle.Render(info)
	case "Running":
		return WarningStyle.Render(info)
	case "Failed", "Error":
		return ErrorStyle.Render(info)
	default:
		return info
	}
}

// KeyValue renders a formatted key-value line with consistent alignment.
//
//	Example output: "  Phase           Ready ✓"
func KeyValue(key, value string) string {
	return fmt.Sprintf("  %s %s", keyLabel.Render(key), value)
}

// SectionHeader renders a bold section heading with a leading blank line.
func SectionHeader(title string) string {
	return "\n" + HeadingStyle.Render("  "+title)
}

// Box wraps content in a rounded-border box using the accent color.
func Box(content string) string {
	return boxBorder.Render(content)
}

// Divider returns a horizontal rule of the given width.
func Divider(width int) string {
	if width <= 0 {
		width = 48
	}
	return SubtleStyle.Render(strings.Repeat("─", width))
}

// Indent prefixes every non-empty line with the given indent level (2 spaces each).
func Indent(s string, level int) string {
	pad := strings.Repeat("  ", level)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

// ValueOrMuted returns the value or a muted placeholder if empty.
func ValueOrMuted(v, placeholder string) string {
	if v == "" {
		return MutedStyle.Render(placeholder)
	}
	return v
}

// StyledCount renders a number; if non-zero it is colored with the given style.
func StyledCount(n int, style lipgloss.Style) string {
	s := fmt.Sprintf("%d", n)
	if n > 0 {
		return style.Render(s)
	}
	return s
}
