package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- spinnerModel tests ---

func TestNewSpinnerModel(t *testing.T) {
	fn := func() (string, error) { return "done", nil }
	m := newSpinnerModel("Loading data", fn)

	if m.title != "Loading data" {
		t.Errorf("title = %q, want 'Loading data'", m.title)
	}
	if m.done {
		t.Error("done should be false initially")
	}
	if m.err != nil {
		t.Errorf("err should be nil initially, got %v", m.err)
	}
	if m.fn == nil {
		t.Error("fn should not be nil")
	}
}

func TestSpinnerModel_Update_Done_Success(t *testing.T) {
	fn := func() (string, error) { return "details", nil }
	m := newSpinnerModel("Loading", fn)

	msg := spinnerDoneMsg{detail: "loaded 5 items", err: nil}
	model, cmd := m.Update(msg)
	sm := model.(spinnerModel)

	if !sm.done {
		t.Error("done should be true after spinnerDoneMsg")
	}
	if sm.detail != "loaded 5 items" {
		t.Errorf("detail = %q, want 'loaded 5 items'", sm.detail)
	}
	if sm.err != nil {
		t.Errorf("err should be nil, got %v", sm.err)
	}
	if cmd == nil {
		t.Error("done msg should return a quit command")
	}
}

func TestSpinnerModel_Update_Done_Error(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Loading", fn)

	testErr := fmt.Errorf("timeout")
	msg := spinnerDoneMsg{detail: "", err: testErr}
	model, cmd := m.Update(msg)
	sm := model.(spinnerModel)

	if !sm.done {
		t.Error("done should be true after error")
	}
	if sm.err == nil {
		t.Error("err should be set")
	}
	if cmd == nil {
		t.Error("error msg should return a quit command")
	}
}

func TestSpinnerModel_Update_CtrlC(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Loading", fn)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)
	sm := model.(spinnerModel)

	if sm.err == nil {
		t.Error("err should be set for cancellation")
	}
	if sm.err.Error() != "cancelled" {
		t.Errorf("err = %q, want 'cancelled'", sm.err.Error())
	}
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestSpinnerModel_View_InProgress(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Loading data", fn)

	view := m.View()
	if !strings.Contains(view, "Loading data") {
		t.Errorf("View in progress should contain title, got %q", view)
	}
}

func TestSpinnerModel_View_Success(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Loading data", fn)
	m.done = true
	m.detail = "5 items"

	view := m.View()
	if !strings.Contains(view, IconCheck) {
		t.Errorf("View on success should contain check icon, got %q", view)
	}
	if !strings.Contains(view, "Loading data") {
		t.Errorf("View on success should contain title, got %q", view)
	}
	if !strings.Contains(view, "5 items") {
		t.Errorf("View on success should contain detail, got %q", view)
	}
}

func TestSpinnerModel_View_Success_NoDetail(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Checking", fn)
	m.done = true

	view := m.View()
	if !strings.Contains(view, IconCheck) {
		t.Errorf("View on success should contain check icon, got %q", view)
	}
	if !strings.Contains(view, "Checking") {
		t.Errorf("View on success should contain title, got %q", view)
	}
}

func TestSpinnerModel_View_Error(t *testing.T) {
	fn := func() (string, error) { return "", nil }
	m := newSpinnerModel("Loading data", fn)
	m.done = true
	m.err = fmt.Errorf("connection refused")

	view := m.View()
	if !strings.Contains(view, IconCross) {
		t.Errorf("View on error should contain cross icon, got %q", view)
	}
	if !strings.Contains(view, "connection refused") {
		t.Errorf("View on error should contain error message, got %q", view)
	}
}

// --- multiStepModel tests ---

func TestNewMultiStepModel(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Run: func() (string, error) { return "", nil }},
		{Title: "Step 2", Run: func() (string, error) { return "", nil }},
		{Title: "Step 3", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Multi Op", steps)

	if m.title != "Multi Op" {
		t.Errorf("title = %q, want 'Multi Op'", m.title)
	}
	if len(m.steps) != 3 {
		t.Errorf("steps len = %d, want 3", len(m.steps))
	}
	if len(m.results) != 3 {
		t.Errorf("results len = %d, want 3", len(m.results))
	}
	if m.current != 0 {
		t.Errorf("current = %d, want 0", m.current)
	}
	if m.done {
		t.Error("done should be false initially")
	}
}

func TestMultiStepModel_Update_StepDone_Success(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Run: func() (string, error) { return "", nil }},
		{Title: "Step 2", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)

	msg := stepDoneMsg{index: 0, detail: "ok", elapsed: 100 * time.Millisecond}
	model, cmd := m.Update(msg)
	sm := model.(multiStepModel)

	if sm.current != 1 {
		t.Errorf("current should advance to 1, got %d", sm.current)
	}
	if sm.done {
		t.Error("done should be false (more steps remain)")
	}
	if sm.results[0].Title != "Step 1" {
		t.Errorf("results[0].Title = %q, want 'Step 1'", sm.results[0].Title)
	}
	if sm.results[0].Detail != "ok" {
		t.Errorf("results[0].Detail = %q, want 'ok'", sm.results[0].Detail)
	}
	if sm.results[0].Err != nil {
		t.Errorf("results[0].Err should be nil, got %v", sm.results[0].Err)
	}
	if cmd == nil {
		t.Error("finishing a step should return a command to run the next one")
	}
}

func TestMultiStepModel_Update_StepDone_LastStep(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)

	msg := stepDoneMsg{index: 0, detail: "complete", elapsed: 50 * time.Millisecond}
	model, cmd := m.Update(msg)
	sm := model.(multiStepModel)

	if !sm.done {
		t.Error("done should be true after last step completes")
	}
	if cmd == nil {
		t.Error("completing all steps should return a quit command")
	}
}

func TestMultiStepModel_Update_StepDone_Error(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Run: func() (string, error) { return "", nil }},
		{Title: "Step 2", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)

	testErr := fmt.Errorf("step failed")
	msg := stepDoneMsg{index: 0, detail: "", err: testErr, elapsed: 10 * time.Millisecond}
	model, cmd := m.Update(msg)
	sm := model.(multiStepModel)

	if !sm.done {
		t.Error("done should be true after step error")
	}
	if sm.results[0].Err == nil {
		t.Error("results[0].Err should be set")
	}
	if cmd == nil {
		t.Error("error should return a quit command")
	}
}

func TestMultiStepModel_Update_CtrlC(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Run: func() (string, error) { return "", nil }},
		{Title: "Step 2", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)
	sm := model.(multiStepModel)

	if !sm.done {
		t.Error("done should be true after ctrl+c")
	}
	if sm.results[sm.current].Err == nil {
		t.Error("current step should have a cancellation error")
	}
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestMultiStepModel_View_InProgress(t *testing.T) {
	steps := []Step{
		{Title: "Step A", Run: func() (string, error) { return "", nil }},
		{Title: "Step B", Run: func() (string, error) { return "", nil }},
		{Title: "Step C", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Operation", steps)
	m.current = 1
	m.results[0] = StepResult{Title: "Step A", Detail: "done", Elapsed: 50 * time.Millisecond}

	view := m.View()
	if !strings.Contains(view, "Operation") {
		t.Errorf("View should contain title 'Operation', got %q", view)
	}
	if !strings.Contains(view, "Step A") {
		t.Errorf("View should contain completed step 'Step A', got %q", view)
	}
	if !strings.Contains(view, IconCheck) {
		t.Errorf("View should contain check icon for completed step, got %q", view)
	}
	if !strings.Contains(view, "Step B") {
		t.Errorf("View should contain current step 'Step B', got %q", view)
	}
	if !strings.Contains(view, "Step C") {
		t.Errorf("View should contain pending step 'Step C', got %q", view)
	}
	if !strings.Contains(view, IconPending) {
		t.Errorf("View should contain pending icon, got %q", view)
	}
	if !strings.Contains(view, "cancel") {
		t.Errorf("View should contain cancel instruction, got %q", view)
	}
}

func TestMultiStepModel_View_Completed(t *testing.T) {
	steps := []Step{
		{Title: "Step A", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)
	m.done = true
	m.current = 1
	m.results[0] = StepResult{Title: "Step A", Detail: "finished", Elapsed: 100 * time.Millisecond}

	view := m.View()
	if strings.Contains(view, "cancel") {
		t.Errorf("View when done should not show cancel instruction, got %q", view)
	}
}

func TestMultiStepModel_View_Error(t *testing.T) {
	steps := []Step{
		{Title: "Step A", Run: func() (string, error) { return "", nil }},
	}
	m := newMultiStepModel("Op", steps)
	m.done = true
	m.current = 1
	m.results[0] = StepResult{Title: "Step A", Err: fmt.Errorf("boom"), Elapsed: 10 * time.Millisecond}

	view := m.View()
	if !strings.Contains(view, IconCross) {
		t.Errorf("View should contain cross icon for failed step, got %q", view)
	}
	if !strings.Contains(view, "boom") {
		t.Errorf("View should contain error message, got %q", view)
	}
}

// --- StepResult --- 

func TestStepResult_Fields(t *testing.T) {
	r := StepResult{
		Title:   "Check deps",
		Detail:  "all present",
		Err:     nil,
		Elapsed: 200 * time.Millisecond,
	}
	if r.Title != "Check deps" {
		t.Errorf("Title = %q, want 'Check deps'", r.Title)
	}
	if r.Detail != "all present" {
		t.Errorf("Detail = %q, want 'all present'", r.Detail)
	}
	if r.Elapsed != 200*time.Millisecond {
		t.Errorf("Elapsed = %v, want 200ms", r.Elapsed)
	}
}
