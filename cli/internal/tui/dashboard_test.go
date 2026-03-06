package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewDashboardModel(t *testing.T) {
	sections := []DashboardSection{
		{Title: "Nodes", Load: func() (string, error) { return "nodes data", nil }},
		{Title: "Apps", Load: func() (string, error) { return "apps data", nil }},
	}
	m := newDashboardModel("Test Dashboard", sections)

	if m.title != "Test Dashboard" {
		t.Errorf("title = %q, want 'Test Dashboard'", m.title)
	}
	if len(m.sections) != 2 {
		t.Errorf("sections len = %d, want 2", len(m.sections))
	}
	if len(m.contents) != 2 {
		t.Errorf("contents len = %d, want 2", len(m.contents))
	}
	if len(m.errors) != 2 {
		t.Errorf("errors len = %d, want 2", len(m.errors))
	}
	if len(m.loading) != 2 {
		t.Errorf("loading len = %d, want 2", len(m.loading))
	}
	if m.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0", m.activeTab)
	}
	if !m.autoRefresh {
		t.Error("autoRefresh should be true")
	}
}

func TestDashboardModel_Init(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "ok", nil }},
	}
	m := newDashboardModel("Title", sections)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a batch command for spinner + section loads")
	}
}

func TestDashboardModel_Update_WindowSize(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	model, cmd := m.Update(msg)
	dm := model.(dashboardModel)

	if dm.width != 120 {
		t.Errorf("width = %d, want 120", dm.width)
	}
	if dm.height != 40 {
		t.Errorf("height = %d, want 40", dm.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil command")
	}
}

func TestDashboardModel_Update_NextTab(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
		{Title: "S2", Load: func() (string, error) { return "", nil }},
		{Title: "S3", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	// Press tab (next tab)
	msg := tea.KeyMsg{Type: tea.KeyTab}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab after tab = %d, want 1", dm.activeTab)
	}

	// Press tab again
	model, _ = dm.Update(msg)
	dm = model.(dashboardModel)
	if dm.activeTab != 2 {
		t.Errorf("activeTab after second tab = %d, want 2", dm.activeTab)
	}

	// Press tab again (should wrap to 0)
	model, _ = dm.Update(msg)
	dm = model.(dashboardModel)
	if dm.activeTab != 0 {
		t.Errorf("activeTab after wrap = %d, want 0", dm.activeTab)
	}
}

func TestDashboardModel_Update_PrevTab(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
		{Title: "S2", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	// Press shift+tab (prev tab) from 0 should wrap
	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab after shift+tab from 0 = %d, want 1", dm.activeTab)
	}
}

func TestDashboardModel_Update_PrevTab_H(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
		{Title: "S2", Load: func() (string, error) { return "", nil }},
		{Title: "S3", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.activeTab = 2

	// 'h' navigates prev
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab after 'h' from 2 = %d, want 1", dm.activeTab)
	}
}

func TestDashboardModel_Update_NextTab_L(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
		{Title: "S2", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	// 'l' navigates next
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab after 'l' = %d, want 1", dm.activeTab)
	}
}

func TestDashboardModel_Update_Quit(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("quit key should return a command")
	}
}

func TestDashboardModel_Update_CtrlC(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestDashboardModel_Update_HelpToggle(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	initial := m.help.ShowAll
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)
	if dm.help.ShowAll == initial {
		t.Error("help.ShowAll should toggle after '?'")
	}
}

func TestDashboardModel_Update_Refresh(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
		{Title: "S2", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, cmd := m.Update(msg)
	dm := model.(dashboardModel)

	for i, l := range dm.loading {
		if !l {
			t.Errorf("loading[%d] should be true after refresh", i)
		}
	}
	if dm.lastRefresh.IsZero() {
		t.Error("lastRefresh should be set after refresh")
	}
	if cmd == nil {
		t.Error("refresh should return commands to load sections")
	}
}

func TestDashboardModel_Update_DataMsg_Success(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.loading[0] = true

	msg := dashboardDataMsg{tab: 0, content: "loaded content", err: nil}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)

	if dm.loading[0] {
		t.Error("loading should be false after data received")
	}
	if dm.contents[0] != "loaded content" {
		t.Errorf("contents[0] = %q, want 'loaded content'", dm.contents[0])
	}
	if dm.errors[0] != nil {
		t.Errorf("errors[0] should be nil, got %v", dm.errors[0])
	}
}

func TestDashboardModel_Update_DataMsg_Error(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.loading[0] = true

	testErr := fmt.Errorf("connection failed")
	msg := dashboardDataMsg{tab: 0, content: "", err: testErr}
	model, _ := m.Update(msg)
	dm := model.(dashboardModel)

	if dm.loading[0] {
		t.Error("loading should be false after error")
	}
	if dm.contents[0] != "" {
		t.Errorf("contents[0] should be empty on error, got %q", dm.contents[0])
	}
	if dm.errors[0] == nil {
		t.Error("errors[0] should be set")
	}
}

func TestDashboardModel_Update_RefreshMsg(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	msg := dashboardRefreshMsg{}
	model, cmd := m.Update(msg)
	dm := model.(dashboardModel)

	for i, l := range dm.loading {
		if !l {
			t.Errorf("loading[%d] should be true after refresh msg", i)
		}
	}
	if cmd == nil {
		t.Error("refresh msg should return commands")
	}
}

func TestDashboardModel_View_Basic(t *testing.T) {
	sections := []DashboardSection{
		{Title: "Nodes", Load: func() (string, error) { return "", nil }},
		{Title: "Apps", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Platform", sections)
	m.contents[0] = "node data here"
	m.width = 80

	view := m.View()
	if !strings.Contains(view, "Platform") {
		t.Errorf("View should contain title 'Platform', got %q", view)
	}
	if !strings.Contains(view, "Nodes") {
		t.Errorf("View should contain tab 'Nodes', got %q", view)
	}
	if !strings.Contains(view, "Apps") {
		t.Errorf("View should contain tab 'Apps', got %q", view)
	}
	if !strings.Contains(view, "node data here") {
		t.Errorf("View should contain active tab content, got %q", view)
	}
}

func TestDashboardModel_View_Error(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.errors[0] = fmt.Errorf("load failed")
	m.width = 80

	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Errorf("View with error should contain 'Error', got %q", view)
	}
}

func TestDashboardModel_View_Loading(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.loading[0] = true
	m.width = 80

	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("View while loading should contain 'Loading', got %q", view)
	}
}

func TestDashboardModel_View_DefaultWidth(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)
	m.contents[0] = "content"
	// width = 0 (default), should use 80

	view := m.View()
	// Separator line should be 80 chars
	if !strings.Contains(view, "─") {
		t.Errorf("View should contain separator line, got %q", view)
	}
}

func TestDashKeyMap_ShortHelp(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	bindings := m.keys.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp should return bindings")
	}
}

func TestDashKeyMap_FullHelp(t *testing.T) {
	sections := []DashboardSection{
		{Title: "S1", Load: func() (string, error) { return "", nil }},
	}
	m := newDashboardModel("Title", sections)

	groups := m.keys.FullHelp()
	if len(groups) == 0 {
		t.Error("FullHelp should return binding groups")
	}
}
