package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Interactive table model tests ---

func TestNewInteractiveTableModel_Basic(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Pods",
		Headers: []string{"NAME", "STATUS", "AGE"},
		Rows: [][]string{
			{"pod-1", "Running", "5m"},
			{"pod-2", "Pending", "2m"},
		},
	}
	m := newInteractiveTableModel(cfg)

	if m.title != "Pods" {
		t.Errorf("title = %q, want 'Pods'", m.title)
	}
	if len(m.headers) != 3 {
		t.Errorf("headers len = %d, want 3", len(m.headers))
	}
	if len(m.allRows) != 2 {
		t.Errorf("allRows len = %d, want 2", len(m.allRows))
	}
	if m.filterMode {
		t.Error("filterMode should be false initially")
	}
	if m.filterText != "" {
		t.Errorf("filterText should be empty, got %q", m.filterText)
	}
	if m.quit {
		t.Error("quit should be false initially")
	}
	if m.selected != nil {
		t.Error("selected should be nil initially")
	}
	if m.detail != "" {
		t.Errorf("detail should be empty, got %q", m.detail)
	}
}

func TestNewInteractiveTableModel_Empty(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Empty",
		Headers: []string{"NAME"},
		Rows:    [][]string{},
	}
	m := newInteractiveTableModel(cfg)
	if len(m.allRows) != 0 {
		t.Errorf("allRows should be empty, got %d", len(m.allRows))
	}
}

func TestInteractiveTableModel_Init(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestInteractiveTableModel_Update_Quit(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	model, cmd := m.Update(msg)
	tm := model.(interactiveTableModel)

	if !tm.quit {
		t.Error("quit should be true after 'q'")
	}
	if cmd == nil {
		t.Error("quit should return a command")
	}
}

func TestInteractiveTableModel_Update_CtrlC(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)
	tm := model.(interactiveTableModel)

	if !tm.quit {
		t.Error("quit should be true after ctrl+c")
	}
	if cmd == nil {
		t.Error("ctrl+c should return a command")
	}
}

func TestInteractiveTableModel_Update_Filter(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}, {"gamma"}},
	}
	m := newInteractiveTableModel(cfg)

	// Enter filter mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if !tm.filterMode {
		t.Error("filterMode should be true after '/'")
	}
}

func TestInteractiveTableModel_Update_FilterInput(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}, {"gamma"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true

	// Type a character
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.filterText != "a" {
		t.Errorf("filterText = %q, want 'a'", tm.filterText)
	}
	// Should filter to rows containing "a"
	rows := tm.table.Rows()
	for _, r := range rows {
		found := false
		for _, cell := range r {
			if strings.Contains(strings.ToLower(cell), "a") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("filtered row %v should contain 'a'", r)
		}
	}
}

func TestInteractiveTableModel_Update_FilterEsc(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true
	m.filterText = "alp"

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.filterMode {
		t.Error("filterMode should be false after esc")
	}
	if tm.filterText != "" {
		t.Errorf("filterText should be empty after esc, got %q", tm.filterText)
	}
}

func TestInteractiveTableModel_Update_FilterEnter(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true
	m.filterText = "alp"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.filterMode {
		t.Error("filterMode should be false after enter")
	}
	// Filter text remains (filter is applied)
	if tm.filterText != "alp" {
		t.Errorf("filterText should remain 'alp' after enter, got %q", tm.filterText)
	}
}

func TestInteractiveTableModel_Update_FilterBackspace(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true
	m.filterText = "alp"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.filterText != "al" {
		t.Errorf("filterText after backspace = %q, want 'al'", tm.filterText)
	}
}

func TestInteractiveTableModel_Update_FilterBackspace_Empty(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true
	m.filterText = ""

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.filterText != "" {
		t.Errorf("filterText should still be empty, got %q", tm.filterText)
	}
}

func TestInteractiveTableModel_Update_Escape_NoDetail(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	model, cmd := m.Update(msg)
	tm := model.(interactiveTableModel)

	if !tm.quit {
		t.Error("esc without detail should quit")
	}
	if cmd == nil {
		t.Error("esc should return a quit command")
	}
}

func TestInteractiveTableModel_Update_Escape_WithDetail(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	m.detail = "some detail"
	m.selected = &TableAction{Row: []string{"1"}, Index: 0}

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.quit {
		t.Error("esc with detail should close detail, not quit")
	}
	if tm.detail != "" {
		t.Errorf("detail should be cleared, got %q", tm.detail)
	}
	if tm.selected != nil {
		t.Error("selected should be nil after closing detail")
	}
}

func TestInteractiveTableModel_Update_HelpToggle(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)

	initial := m.help.ShowAll
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.help.ShowAll == initial {
		t.Error("help.ShowAll should toggle after '?'")
	}
}

func TestInteractiveTableModel_Update_WindowSize(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}},
	}
	m := newInteractiveTableModel(cfg)

	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.width != 100 {
		t.Errorf("width = %d, want 100", tm.width)
	}
	if tm.height != 30 {
		t.Errorf("height = %d, want 30", tm.height)
	}
}

func TestInteractiveTableModel_View(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Pod List",
		Headers: []string{"NAME", "STATUS"},
		Rows:    [][]string{{"pod-1", "Running"}, {"pod-2", "Pending"}},
	}
	m := newInteractiveTableModel(cfg)

	view := m.View()
	if !strings.Contains(view, "Pod List") {
		t.Errorf("View should contain title 'Pod List', got %q", view)
	}
	if !strings.Contains(view, "2 items") {
		t.Errorf("View should contain '2 items', got %q", view)
	}
}

func TestInteractiveTableModel_View_FilterMode(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterMode = true
	m.filterText = "pod"

	view := m.View()
	if !strings.Contains(view, "pod") {
		t.Errorf("View in filter mode should contain filter text, got %q", view)
	}
}

func TestInteractiveTableModel_View_FilteredCount(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"alpha"}, {"beta"}, {"gamma"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterText = "a"
	m.applyFilter()

	view := m.View()
	if !strings.Contains(view, "filtered") {
		t.Errorf("View with active filter should mention 'filtered', got %q", view)
	}
}

// --- applyFilter tests ---

func TestApplyFilter_EmptyFilter(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterText = ""
	m.applyFilter()

	if len(m.table.Rows()) != 2 {
		t.Errorf("empty filter should show all rows, got %d", len(m.table.Rows()))
	}
}

func TestApplyFilter_MatchingSome(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME", "STATUS"},
		Rows:    [][]string{{"alpha", "running"}, {"beta", "pending"}, {"gamma", "running"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterText = "running"
	m.applyFilter()

	rows := m.table.Rows()
	if len(rows) != 2 {
		t.Errorf("filter 'running' should match 2 rows, got %d", len(rows))
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterText = "zzz"
	m.applyFilter()

	if len(m.table.Rows()) != 0 {
		t.Errorf("filter 'zzz' should match 0 rows, got %d", len(m.table.Rows()))
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"NAME"},
		Rows:    [][]string{{"Alpha"}, {"beta"}},
	}
	m := newInteractiveTableModel(cfg)
	m.filterText = "ALPHA"
	m.applyFilter()

	if len(m.table.Rows()) != 1 {
		t.Errorf("filter should be case-insensitive, got %d rows", len(m.table.Rows()))
	}
}

// --- stripAnsi tests ---

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no ansi", "no ansi"},
		{"", ""},
		{"\x1b[38;5;99mpurple\x1b[0m text", "purple text"},
	}
	for _, tt := range tests {
		got := stripAnsi(tt.input)
		if got != tt.want {
			t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- overlayWidth tests ---

func TestOverlayWidth(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)

	// Normal width
	m.width = 120
	w := m.overlayWidth()
	if w != 100 { // capped at 100
		t.Errorf("overlayWidth for width=120 should be 100, got %d", w)
	}

	// Small width
	m.width = 30
	w = m.overlayWidth()
	if w != 40 { // minimum 40
		t.Errorf("overlayWidth for width=30 should be 40, got %d", w)
	}

	// Medium width
	m.width = 80
	w = m.overlayWidth()
	expected := 80 - 8
	if w != expected {
		t.Errorf("overlayWidth for width=80 should be %d, got %d", expected, w)
	}
}

// --- tableKeyMap tests ---

func TestTableKeyMap_ShortHelp(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	bindings := m.keys.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp should return bindings")
	}
}

func TestTableKeyMap_FullHelp(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	groups := m.keys.FullHelp()
	if len(groups) == 0 {
		t.Error("FullHelp should return binding groups")
	}
}

// --- clearCopyMsg ---

func TestInteractiveTableModel_Update_ClearCopyMsg(t *testing.T) {
	cfg := InteractiveTableConfig{
		Title:   "Test",
		Headers: []string{"A"},
		Rows:    [][]string{{"1"}},
	}
	m := newInteractiveTableModel(cfg)
	m.copyStatus = "Copied!"
	m.width = 80
	m.height = 24

	msg := clearCopyMsg{}
	model, _ := m.Update(msg)
	tm := model.(interactiveTableModel)

	if tm.copyStatus != "" {
		t.Errorf("copyStatus should be cleared, got %q", tm.copyStatus)
	}
}

// --- TableAction struct ---

func TestTableAction_Fields(t *testing.T) {
	action := TableAction{
		Row:   []string{"pod-1", "Running"},
		Index: 2,
	}
	if action.Index != 2 {
		t.Errorf("Index = %d, want 2", action.Index)
	}
	if len(action.Row) != 2 {
		t.Errorf("Row len = %d, want 2", len(action.Row))
	}
	if action.Row[0] != "pod-1" {
		t.Errorf("Row[0] = %q, want 'pod-1'", action.Row[0])
	}
}

// --- min utility ---

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Errorf("min(3,5) = %d, want 3", min(3, 5))
	}
	if min(5, 3) != 3 {
		t.Errorf("min(5,3) = %d, want 3", min(5, 3))
	}
	if min(4, 4) != 4 {
		t.Errorf("min(4,4) = %d, want 4", min(4, 4))
	}
}
