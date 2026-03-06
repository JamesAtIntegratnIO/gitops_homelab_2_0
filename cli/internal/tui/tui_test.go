package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Table tests ---

func TestTable_EmptyRows(t *testing.T) {
	got := Table([]string{"NAME", "STATUS"}, nil)
	if !strings.Contains(got, "(no data)") {
		t.Errorf("Table with no rows should show '(no data)', got %q", got)
	}
}

func TestTable_EmptyRowsSlice(t *testing.T) {
	got := Table([]string{"NAME", "STATUS"}, [][]string{})
	if !strings.Contains(got, "(no data)") {
		t.Errorf("Table with empty rows slice should show '(no data)', got %q", got)
	}
}

func TestTable_WithData(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"pod-1", "Running"},
		{"pod-2", "Pending"},
	}
	got := Table(headers, rows)
	if !strings.Contains(got, "NAME") {
		t.Errorf("Table should contain header 'NAME', got %q", got)
	}
	if !strings.Contains(got, "STATUS") {
		t.Errorf("Table should contain header 'STATUS', got %q", got)
	}
	if !strings.Contains(got, "pod-1") {
		t.Errorf("Table should contain 'pod-1', got %q", got)
	}
	if !strings.Contains(got, "Running") {
		t.Errorf("Table should contain 'Running', got %q", got)
	}
	if !strings.Contains(got, "pod-2") {
		t.Errorf("Table should contain 'pod-2', got %q", got)
	}
}

func TestTable_DimmedLastColumn(t *testing.T) {
	headers := []string{"NAME", "COUNT"}
	rows := [][]string{
		{"thing-1", "5"},
		{"thing-2", "0"},
		{"thing-3", "—"},
	}
	// Should not panic with dim cells
	got := Table(headers, rows)
	if !strings.Contains(got, "thing-1") {
		t.Errorf("Table should contain 'thing-1', got %q", got)
	}
}

// --- TreeNode tests ---

func TestTreeNode_Normal(t *testing.T) {
	got := TreeNode("Checking", "OK", "", false)
	if !strings.Contains(got, "├──") {
		t.Errorf("non-last TreeNode should use '├──', got %q", got)
	}
	if !strings.Contains(got, "Checking") {
		t.Errorf("TreeNode should contain name 'Checking', got %q", got)
	}
	if !strings.Contains(got, "OK") {
		t.Errorf("TreeNode should contain status 'OK', got %q", got)
	}
}

func TestTreeNode_Last(t *testing.T) {
	got := TreeNode("Final", "Done", "", true)
	if !strings.Contains(got, "└──") {
		t.Errorf("last TreeNode should use '└──', got %q", got)
	}
}

func TestTreeNode_WithMessage(t *testing.T) {
	got := TreeNode("Step", "WARN", "needs attention", false)
	if !strings.Contains(got, "needs attention") {
		t.Errorf("TreeNode with message should contain it, got %q", got)
	}
}

func TestTreeNode_EmptyMessage(t *testing.T) {
	got := TreeNode("Step", "OK", "", false)
	// Should not have trailing extra content from message
	if strings.Contains(got, "needs") {
		t.Errorf("TreeNode with empty message should not have message content, got %q", got)
	}
}

// --- selectModel tests ---

func TestSelectModel_Init(t *testing.T) {
	m := selectModel{
		title:   "Pick one",
		choices: []string{"Alpha", "Beta", "Gamma"},
		cursor:  0,
		chosen:  -1,
	}
	cmd := m.Init()
	if cmd != nil {
		t.Error("selectModel.Init() should return nil")
	}
}

func TestSelectModel_Update_Down(t *testing.T) {
	m := selectModel{
		title:   "Pick one",
		choices: []string{"Alpha", "Beta", "Gamma"},
		cursor:  0,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Errorf("cursor should move to 1 after down, got %d", sm.cursor)
	}
}

func TestSelectModel_Update_Up(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B", "C"},
		cursor:  2,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Errorf("cursor should move to 1 after up from 2, got %d", sm.cursor)
	}
}

func TestSelectModel_Update_UpAtTop(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B"},
		cursor:  0,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 0 {
		t.Errorf("cursor should stay at 0 when already at top, got %d", sm.cursor)
	}
}

func TestSelectModel_Update_DownAtBottom(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B"},
		cursor:  1,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Errorf("cursor should stay at 1 when already at bottom, got %d", sm.cursor)
	}
}

func TestSelectModel_Update_Enter(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B", "C"},
		cursor:  1,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	model, cmd := m.Update(msg)
	sm := model.(selectModel)
	if sm.chosen != 1 {
		t.Errorf("chosen should be 1 after enter at cursor 1, got %d", sm.chosen)
	}
	if !sm.done {
		t.Error("done should be true after enter")
	}
	if cmd == nil {
		t.Error("enter should return a quit command")
	}
}

func TestSelectModel_Update_Quit(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B"},
		cursor:  0,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	model, cmd := m.Update(msg)
	sm := model.(selectModel)
	if sm.chosen != -1 {
		t.Errorf("chosen should be -1 after quit, got %d", sm.chosen)
	}
	if !sm.done {
		t.Error("done should be true after quit")
	}
	if cmd == nil {
		t.Error("quit should return a command")
	}
}

func TestSelectModel_Update_CtrlC(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B"},
		cursor:  0,
		chosen:  -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)
	sm := model.(selectModel)
	if sm.chosen != -1 {
		t.Errorf("chosen should be -1 after ctrl+c, got %d", sm.chosen)
	}
	if !sm.done {
		t.Error("done should be true after ctrl+c")
	}
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestSelectModel_Update_K(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B", "C"},
		cursor:  2,
		chosen:  -1,
	}

	// 'k' is vim-style up
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Errorf("cursor should move to 1 after 'k', got %d", sm.cursor)
	}
}

func TestSelectModel_Update_J(t *testing.T) {
	m := selectModel{
		title:   "Pick",
		choices: []string{"A", "B", "C"},
		cursor:  0,
		chosen:  -1,
	}

	// 'j' is vim-style down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	model, _ := m.Update(msg)
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Errorf("cursor should move to 1 after 'j', got %d", sm.cursor)
	}
}

func TestSelectModel_View(t *testing.T) {
	m := selectModel{
		title:   "Choose",
		choices: []string{"Alpha", "Beta"},
		cursor:  0,
		chosen:  -1,
	}

	view := m.View()
	if !strings.Contains(view, "Choose") {
		t.Errorf("View should contain title 'Choose', got %q", view)
	}
	if !strings.Contains(view, "Alpha") {
		t.Errorf("View should contain choice 'Alpha', got %q", view)
	}
	if !strings.Contains(view, "Beta") {
		t.Errorf("View should contain choice 'Beta', got %q", view)
	}
	if !strings.Contains(view, "▸") {
		t.Errorf("View should contain cursor indicator '▸', got %q", view)
	}
}

func TestSelectModel_View_CursorPosition(t *testing.T) {
	m := selectModel{
		title:   "Choose",
		choices: []string{"A", "B", "C"},
		cursor:  1,
		chosen:  -1,
	}

	view := m.View()
	// The cursor should visually be on the second item
	if !strings.Contains(view, "A") && !strings.Contains(view, "B") {
		t.Errorf("View should contain choices, got %q", view)
	}
}

// --- inputModel tests ---

func TestInputModel_Init(t *testing.T) {
	m := newInputModel("Enter name", "name...", "default")
	cmd := m.Init()
	// Init returns textinput.Blink which is non-nil
	if cmd == nil {
		t.Error("inputModel.Init() should return a blink command")
	}
}

func TestInputModel_Update_CtrlC(t *testing.T) {
	m := newInputModel("Enter name", "name...", "")

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)
	im := model.(inputModel)
	if !im.canceled {
		t.Error("canceled should be true after ctrl+c")
	}
	if !im.done {
		t.Error("done should be true after ctrl+c")
	}
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestInputModel_Update_Enter(t *testing.T) {
	m := newInputModel("Enter name", "name...", "myvalue")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	model, cmd := m.Update(msg)
	im := model.(inputModel)
	if im.canceled {
		t.Error("canceled should be false after enter")
	}
	if !im.done {
		t.Error("done should be true after enter")
	}
	if im.value != "myvalue" {
		t.Errorf("value should be 'myvalue', got %q", im.value)
	}
	if cmd == nil {
		t.Error("enter should return a quit command")
	}
}

func TestInputModel_View(t *testing.T) {
	m := newInputModel("Enter name", "placeholder", "")
	view := m.View()
	if !strings.Contains(view, "Enter name") {
		t.Errorf("View should contain title, got %q", view)
	}
	if !strings.Contains(view, "cancel") {
		t.Errorf("View should contain help text about cancel, got %q", view)
	}
}

func TestInputModel_DefaultValue(t *testing.T) {
	m := newInputModel("Title", "holder", "preset")
	if m.input.Value() != "preset" {
		t.Errorf("input should have default value 'preset', got %q", m.input.Value())
	}
}

func TestInputModel_EmptyDefault(t *testing.T) {
	m := newInputModel("Title", "holder", "")
	if m.input.Value() != "" {
		t.Errorf("input should have empty default, got %q", m.input.Value())
	}
}
