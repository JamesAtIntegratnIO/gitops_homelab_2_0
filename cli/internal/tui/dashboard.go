package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardSection represents one tab/section of the dashboard.
type DashboardSection struct {
	Title string
	// Load fetches the data for this section. Returns rendered content.
	Load func() (string, error)
}

type dashKeyMap struct {
	NextTab  key.Binding
	PrevTab  key.Binding
	Refresh  key.Binding
	Quit     key.Binding
	Help     key.Binding
}

func (k dashKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PrevTab, k.NextTab, k.Refresh, k.Quit}
}

func (k dashKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.PrevTab, k.NextTab, k.Refresh, k.Help, k.Quit}}
}

type dashboardRefreshMsg struct{}
type dashboardDataMsg struct {
	tab     int
	content string
	err     error
}

type dashboardModel struct {
	sections    []DashboardSection
	contents    []string
	errors      []error
	activeTab   int
	spinner     spinner.Model
	loading     []bool
	keys        dashKeyMap
	help        help.Model
	width       int
	height      int
	lastRefresh time.Time
	autoRefresh bool
	title       string
}

func newDashboardModel(title string, sections []DashboardSection) dashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorAccent)

	km := dashKeyMap{
		NextTab: key.NewBinding(
			key.WithKeys("tab", "l", "right"),
			key.WithHelp("tab/→", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab", "h", "left"),
			key.WithHelp("shift+tab/←", "prev tab"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ColorAccent)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(ColorGray)

	return dashboardModel{
		title:       title,
		sections:    sections,
		contents:    make([]string, len(sections)),
		errors:      make([]error, len(sections)),
		loading:     make([]bool, len(sections)),
		spinner:     s,
		keys:        km,
		help:        h,
		autoRefresh: true,
	}
}

func (m dashboardModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	// Load all sections
	for i := range m.sections {
		cmds = append(cmds, m.loadSection(i))
	}
	return tea.Batch(cmds...)
}

func (m dashboardModel) loadSection(index int) tea.Cmd {
	section := m.sections[index]
	return func() tea.Msg {
		content, err := section.Load()
		return dashboardDataMsg{tab: index, content: content, err: err}
	}
}

func (m dashboardModel) refreshAll() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.sections {
		cmds = append(cmds, m.loadSection(i))
	}
	return tea.Batch(cmds...)
}

func (m dashboardModel) autoRefreshCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return dashboardRefreshMsg{}
	})
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case dashboardRefreshMsg:
		for i := range m.loading {
			m.loading[i] = true
		}
		m.lastRefresh = time.Now()
		return m, tea.Batch(m.refreshAll(), m.autoRefreshCmd())

	case dashboardDataMsg:
		m.loading[msg.tab] = false
		if msg.err != nil {
			m.errors[msg.tab] = msg.err
			m.contents[msg.tab] = ""
		} else {
			m.contents[msg.tab] = msg.content
			m.errors[msg.tab] = nil
		}
		if m.lastRefresh.IsZero() {
			m.lastRefresh = time.Now()
		}
		// Start auto-refresh after first load completes
		allDone := true
		for _, l := range m.loading {
			if l {
				allDone = false
				break
			}
		}
		if allDone && m.autoRefresh {
			return m, m.autoRefreshCmd()
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % len(m.sections)
		case key.Matches(msg, m.keys.PrevTab):
			m.activeTab = (m.activeTab - 1 + len(m.sections)) % len(m.sections)
		case key.Matches(msg, m.keys.Refresh):
			for i := range m.loading {
				m.loading[i] = true
			}
			m.lastRefresh = time.Now()
			return m, m.refreshAll()
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m dashboardModel) View() string {
	var sb strings.Builder

	// Title bar
	titleStyle := BannerStyle.MarginBottom(1)

	refreshInfo := ""
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Round(time.Second)
		refreshInfo = MutedStyle.Render(fmt.Sprintf("  refreshed %s ago", ago))
	}
	sb.WriteString(titleStyle.Render(m.title) + refreshInfo + "\n\n")

	// Tab bar
	activeTabStyle := BannerStyle
	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Padding(0, 2)
	tabSep := SubtleStyle.Render(" │ ")

	var tabs []string
	for i, section := range m.sections {
		label := section.Title
		if m.loading[i] {
			label = m.spinner.View() + " " + label
		}
		if i == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	_ = activeTabStyle
	sb.WriteString(strings.Join(tabs, tabSep))
	sb.WriteString("\n")

	// Separator line
	lineWidth := m.width
	if lineWidth == 0 {
		lineWidth = 80
	}
	sb.WriteString(SubtleStyle.Render(strings.Repeat("─", lineWidth)))
	sb.WriteString("\n\n")

	// Content
	if m.activeTab < len(m.contents) {
		if m.errors[m.activeTab] != nil {
			sb.WriteString(ErrorStyle.Render("  Error: " + m.errors[m.activeTab].Error()))
		} else if m.contents[m.activeTab] == "" && m.loading[m.activeTab] {
			sb.WriteString(fmt.Sprintf("  %s Loading %s...\n", m.spinner.View(), m.sections[m.activeTab].Title))
		} else {
			sb.WriteString(m.contents[m.activeTab])
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(m.help.View(m.keys))

	return sb.String()
}

// RunDashboard launches a full-screen tabbed dashboard. Blocks until quit.
func RunDashboard(title string, sections []DashboardSection) error {
	if !IsInteractive() {
		// Non-interactive: just print all sections
		fmt.Println(title)
		for _, s := range sections {
			fmt.Printf("\n--- %s ---\n", s.Title)
			content, err := s.Load()
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Println(content)
			}
		}
		return nil
	}

	m := newDashboardModel(title, sections)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
