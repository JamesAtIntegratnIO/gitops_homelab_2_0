package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Table renders a styled table with headers and rows using lipgloss/table.
func Table(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return DimStyle.Render("  (no data)")
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().
		Padding(0, 1)

	dimCellStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(ColorGray)

	borderStyle := lipgloss.NewStyle().
		Foreground(ColorSubtle)

	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			// Dim the last column if it looks like a count or status placeholder
			if col == len(headers)-1 {
				if len(rows) > 0 && row-1 < len(rows) && row > 0 {
					cell := rows[row-1][col]
					if cell == "—" || cell == "0" {
						return dimCellStyle
					}
				}
			}
			return cellStyle
		})

	return t.Render()
}

// TreeNode renders a tree-style status display.
func TreeNode(name, status, message string, isLast bool) string {
	prefix := SubtleStyle.Render("├── ")
	if isLast {
		prefix = SubtleStyle.Render("└── ")
	}
	line := fmt.Sprintf("  %s%s  %s", prefix, name, status)
	if message != "" {
		line += "  " + MutedStyle.Render(message)
	}
	return line
}

// Confirm prompts for y/n confirmation. Returns true if confirmed.
func Confirm(prompt string) (bool, error) {
	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, nil
	}
	return strings.ToLower(strings.TrimSpace(response)) == "y", nil
}

// --- Select model for interactive selection ---

type selectModel struct {
	title   string
	choices []string
	cursor  int
	chosen  int
	done    bool
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.chosen = -1
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = m.cursor
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render(m.title) + "\n\n")
	for i, choice := range m.choices {
		cursor := "  "
		style := lipgloss.NewStyle()
		if m.cursor == i {
			cursor = SelectedStyle.Render("▸ ")
			style = SelectedStyle
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(choice)))
	}
	sb.WriteString(DimStyle.Render("\n↑/↓ navigate • enter select • q quit"))
	return sb.String()
}

// Select presents an interactive selection list. Returns the index of the chosen item, or -1 if cancelled.
func Select(title string, choices []string) (int, error) {
	m := selectModel{title: title, choices: choices, chosen: -1}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return -1, err
	}
	return result.(selectModel).chosen, nil
}

// --- Text input model ---

type inputModel struct {
	title    string
	input    textinput.Model
	done     bool
	value    string
	canceled bool
}

func newInputModel(title, placeholder, defaultVal string) inputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	if defaultVal != "" {
		ti.SetValue(defaultVal)
	}
	return inputModel{title: title, input: ti}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			m.value = m.input.Value()
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	return fmt.Sprintf("%s\n\n%s\n\n%s",
		TitleStyle.Render(m.title),
		m.input.View(),
		DimStyle.Render("enter confirm • ctrl+c cancel"),
	)
}

// Input presents an interactive text input. Returns empty string if cancelled.
func Input(title, placeholder, defaultVal string) (string, error) {
	m := newInputModel(title, placeholder, defaultVal)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	rm := result.(inputModel)
	if rm.canceled {
		return "", nil
	}
	return rm.value, nil
}

// IsInteractive returns true if stdin is a terminal (not piped).
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
