package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	btable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableAction is returned when the user presses enter on a row.
type TableAction struct {
	// Row is the selected row data.
	Row []string
	// Index is the row index.
	Index int
}

// InteractiveTableConfig configures an interactive table.
type InteractiveTableConfig struct {
	Title   string
	Headers []string
	Rows    [][]string
	// OnSelect is called when enter is pressed. If nil, the table exits on enter.
	// Return a string to show as detail below the table, or empty to just select.
	OnSelect func(row []string, index int) string
	// Actions are extra key bindings shown in help. Key is the key, value is description.
	Actions map[string]string
}

// tableKeyMap extends the default table keys with our additions.
type tableKeyMap struct {
	btable.KeyMap
	Quit   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Filter key.Binding
	Help   key.Binding
}

func (k tableKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LineUp, k.LineDown, k.Enter, k.Filter, k.Quit}
}

func (k tableKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LineUp, k.LineDown, k.PageUp, k.PageDown},
		{k.GotoTop, k.GotoBottom},
		{k.Enter, k.Filter, k.Escape, k.Quit},
	}
}

type interactiveTableModel struct {
	title    string
	table    btable.Model
	keys     tableKeyMap
	help     help.Model
	detail   string
	detailVP viewport.Model
	selected *TableAction
	quit     bool
	width    int
	height   int
	// filtering
	filterMode  bool
	filterText  string
	allRows     []btable.Row
	headers     []string
	onSelect    func([]string, int) string
}

func newInteractiveTableModel(cfg InteractiveTableConfig) interactiveTableModel {
	// Build columns with auto-sizing
	widths := make([]int, len(cfg.Headers))
	for i, h := range cfg.Headers {
		widths[i] = len(h) + 2
	}
	for _, row := range cfg.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell)+2 > widths[i] {
				widths[i] = len(cell) + 2
			}
		}
	}

	columns := make([]btable.Column, len(cfg.Headers))
	for i, h := range cfg.Headers {
		columns[i] = btable.Column{Title: h, Width: widths[i]}
	}

	rows := make([]btable.Row, len(cfg.Rows))
	for i, r := range cfg.Rows {
		rows[i] = btable.Row(r)
	}

	t := btable.New(
		btable.WithColumns(columns),
		btable.WithRows(rows),
		btable.WithFocused(true),
		btable.WithHeight(min(len(cfg.Rows)+1, 20)),
	)

	s := btable.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 1)
	s.Cell = lipgloss.NewStyle().
		Padding(0, 1)
	t.SetStyles(s)

	km := tableKeyMap{
		KeyMap: btable.DefaultKeyMap(),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	return interactiveTableModel{
		title:    cfg.Title,
		table:    t,
		keys:     km,
		help:     h,
		allRows:  rows,
		headers:  cfg.Headers,
		onSelect: cfg.OnSelect,
	}
}

func (m interactiveTableModel) Init() tea.Cmd {
	return nil
}

func (m interactiveTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		// Resize table height to fill window (title + help + padding)
		tableHeight := msg.Height - 6
		if tableHeight < 3 {
			tableHeight = 3
		}
		m.table.SetHeight(tableHeight)
		return m, nil

	case tea.KeyMsg:
		// Filter mode input handling
		if m.filterMode {
			switch msg.String() {
			case "esc":
				m.filterMode = false
				m.filterText = ""
				m.table.SetRows(m.allRows)
				return m, nil
			case "enter":
				m.filterMode = false
				return m, nil
			case "backspace":
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					m.applyFilter()
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.filterText += msg.String()
					m.applyFilter()
				}
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Enter):
			if m.detail != "" {
				// Enter while detail is open does nothing
				return m, nil
			}
			row := m.table.SelectedRow()
			if row != nil {
				m.selected = &TableAction{
					Row:   []string(row),
					Index: m.table.Cursor(),
				}
				if m.onSelect != nil {
					m.detail = m.onSelect(m.selected.Row, m.selected.Index)
					if m.detail != "" {
						m.initDetailViewport()
						return m, nil
					}
				}
				return m, tea.Quit
			}

		case key.Matches(msg, m.keys.Escape):
			if m.detail != "" {
				m.detail = ""
				m.selected = nil
				return m, nil
			}
			m.quit = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Filter):
			m.filterMode = true
			m.filterText = ""
			return m, nil

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	}

	// Route keys to viewport when detail overlay is open
	if m.detail != "" {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *interactiveTableModel) applyFilter() {
	if m.filterText == "" {
		m.table.SetRows(m.allRows)
		return
	}
	filter := strings.ToLower(m.filterText)
	var filtered []btable.Row
	for _, row := range m.allRows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), filter) {
				filtered = append(filtered, row)
				break
			}
		}
	}
	m.table.SetRows(filtered)
	m.table.SetCursor(0)
}

// initDetailViewport sets up the viewport sized for the current terminal.
func (m *interactiveTableModel) initDetailViewport() {
	// Content width inside the overlay (account for border + padding: 2 border + 2*2 padding = 6)
	contentW := m.overlayWidth() - 6
	if contentW < 20 {
		contentW = 20
	}

	// Max viewport height (leave room for border + padding + footer)
	maxH := m.height - 8
	if maxH < 5 {
		maxH = 5
	}

	// Wrap the detail text to contentW and set it in the viewport
	wrapped := lipgloss.NewStyle().Width(contentW).Render(m.detail)
	footer := DimStyle.Render("esc to close · ↑↓ scroll")
	fullContent := wrapped + "\n\n" + footer

	lines := strings.Count(fullContent, "\n") + 1
	vpHeight := lines
	if vpHeight > maxH {
		vpHeight = maxH
	}

	m.detailVP = viewport.New(contentW, vpHeight)
	m.detailVP.SetContent(fullContent)
}

func (m interactiveTableModel) overlayWidth() int {
	w := m.width - 8
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	return w
}

func (m interactiveTableModel) View() string {
	var sb strings.Builder

	// Title
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1).
		Render(m.title)
	sb.WriteString(titleBar)
	sb.WriteString("\n")

	// Filter indicator
	if m.filterMode {
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)
		sb.WriteString(filterStyle.Render(fmt.Sprintf("  / %s▌", m.filterText)))
		sb.WriteString("\n")
	}

	// Table
	sb.WriteString(m.table.View())
	sb.WriteString("\n")

	// Row count
	rowCount := fmt.Sprintf("  %d items", len(m.table.Rows()))
	if m.filterText != "" {
		rowCount = fmt.Sprintf("  %d/%d items (filtered)", len(m.table.Rows()), len(m.allRows))
	}
	sb.WriteString(DimStyle.Render(rowCount))
	sb.WriteString("\n")

	// Help
	sb.WriteString(m.help.View(m.keys))

	base := sb.String()

	// Detail overlay — render as a centered modal on top of the table
	if m.detail != "" {
		w := m.overlayWidth()

		// Scroll indicator
		scrollInfo := ""
		if m.detailVP.TotalLineCount() > m.detailVP.VisibleLineCount() {
			scrollInfo = DimStyle.Render(fmt.Sprintf(" %d%%", int(m.detailVP.ScrollPercent()*100)))
		}

		overlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Background(lipgloss.Color("235")).
			Padding(1, 2).
			Width(w).
			Render(m.detailVP.View() + scrollInfo)

		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	return base
}

// InteractiveTable runs a full-screen interactive table. Returns the selected row or nil if cancelled.
// Falls back to static Table() if not running in a terminal.
func InteractiveTable(cfg InteractiveTableConfig) (*TableAction, error) {
	if !IsInteractive() {
		// Non-interactive fallback: print static table
		fmt.Println(Table(cfg.Headers, cfg.Rows))
		return nil, nil
	}

	model := newInteractiveTableModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, err
	}

	rm := result.(interactiveTableModel)
	if rm.quit {
		return nil, nil
	}
	return rm.selected, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
