package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step represents one step in a multi-step operation.
type Step struct {
	Title string
	Run   func() (string, error)
}

// StepResult holds the result of a step execution.
type StepResult struct {
	Title   string
	Detail  string
	Err     error
	Elapsed time.Duration
}

// --- Single spinner model ---

type spinnerDoneMsg struct {
	detail string
	err    error
}

type spinnerModel struct {
	spinner spinner.Model
	title   string
	detail  string
	done    bool
	err     error
	fn      func() (string, error)
}

func newSpinnerModel(title string, fn func() (string, error)) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return spinnerModel{
		spinner: s,
		title:   title,
		fn:      fn,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		detail, err := m.fn()
		return spinnerDoneMsg{detail: detail, err: err}
	})
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerDoneMsg:
		m.done = true
		m.detail = msg.detail
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("  %s %s  %s\n",
				ErrorStyle.Render("✗"),
				m.title,
				ErrorStyle.Render(m.err.Error()))
		}
		result := fmt.Sprintf("  %s %s", SuccessStyle.Render("✓"), m.title)
		if m.detail != "" {
			result += fmt.Sprintf("  %s", DimStyle.Render(m.detail))
		}
		return result + "\n"
	}
	return fmt.Sprintf("  %s %s\n", m.spinner.View(), m.title)
}

// Spin runs an operation with a spinner. Returns the detail string and error.
// Falls back to simple output if not interactive.
func Spin(title string, fn func() (string, error)) (string, error) {
	if !IsInteractive() {
		fmt.Printf("  %s... ", title)
		detail, err := fn()
		if err != nil {
			fmt.Println(ErrorStyle.Render(IconCross + " " + err.Error()))
		} else {
			fmt.Println(SuccessStyle.Render(IconCheck))
		}
		return detail, err
	}

	m := newSpinnerModel(title, fn)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	rm := result.(spinnerModel)
	return rm.detail, rm.err
}

// --- Multi-step spinner model ---

type stepDoneMsg struct {
	index   int
	detail  string
	err     error
	elapsed time.Duration
}

type multiStepModel struct {
	spinner  spinner.Model
	steps    []Step
	results  []StepResult
	current  int
	done     bool
	title    string
	started  time.Time
}

func newMultiStepModel(title string, steps []Step) multiStepModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return multiStepModel{
		spinner: s,
		steps:   steps,
		results: make([]StepResult, len(steps)),
		title:   title,
	}
}

func (m multiStepModel) Init() tea.Cmd {
	m.started = time.Now()
	return tea.Batch(m.spinner.Tick, m.runStep(0))
}

func (m multiStepModel) runStep(index int) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		detail, err := m.steps[index].Run()
		return stepDoneMsg{
			index:   index,
			detail:  detail,
			err:     err,
			elapsed: time.Since(start),
		}
	}
}

func (m multiStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stepDoneMsg:
		m.results[msg.index] = StepResult{
			Title:   m.steps[msg.index].Title,
			Detail:  msg.detail,
			Err:     msg.err,
			Elapsed: msg.elapsed,
		}
		m.current = msg.index + 1

		// If step failed or all done, finish
		if msg.err != nil || m.current >= len(m.steps) {
			m.done = true
			return m, tea.Quit
		}

		// Run next step
		return m, m.runStep(m.current)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.results[m.current] = StepResult{
				Title: m.steps[m.current].Title,
				Err:   fmt.Errorf("cancelled"),
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m multiStepModel) View() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render(m.title))
	sb.WriteString("\n\n")

	for i, step := range m.steps {
		if i < m.current {
			// Completed step
			r := m.results[i]
			if r.Err != nil {
				sb.WriteString(fmt.Sprintf("  %s %s  %s\n",
					ErrorStyle.Render(IconCross),
					step.Title,
					ErrorStyle.Render(r.Err.Error())))
			} else {
				elapsed := MutedStyle.Render(fmt.Sprintf("(%s)", r.Elapsed.Round(time.Millisecond)))
				detail := ""
				if r.Detail != "" {
					detail = MutedStyle.Render(" " + r.Detail)
				}
				sb.WriteString(fmt.Sprintf("  %s %s %s%s\n",
					SuccessStyle.Render(IconCheck),
					step.Title,
					elapsed,
					detail))
			}
		} else if i == m.current && !m.done {
			// Active step
			sb.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), step.Title))
		} else {
			// Pending step
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				MutedStyle.Render(IconPending),
				MutedStyle.Render(step.Title)))
		}
	}

	if !m.done {
		sb.WriteString(MutedStyle.Render("\n  ctrl+c to cancel"))
	}

	return sb.String()
}

// RunSteps executes a sequence of steps with animated spinners.
// Stops on first error. Returns all results.
func RunSteps(title string, steps []Step) ([]StepResult, error) {
	if !IsInteractive() {
		// Non-interactive fallback
		fmt.Println(title)
		results := make([]StepResult, len(steps))
		for i, step := range steps {
			fmt.Printf("  %s... ", step.Title)
			start := time.Now()
			detail, err := step.Run()
			elapsed := time.Since(start)
			results[i] = StepResult{Title: step.Title, Detail: detail, Err: err, Elapsed: elapsed}
			if err != nil {
				fmt.Println(ErrorStyle.Render(IconCross + " " + err.Error()))
				return results, err
			}
			fmt.Println(SuccessStyle.Render(IconCheck))
			_ = detail
		}
		return results, nil
	}

	m := newMultiStepModel(title, steps)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, err
	}
	rm := result.(multiStepModel)

	// Check for step errors
	for _, r := range rm.results {
		if r.Err != nil {
			return rm.results, r.Err
		}
	}
	return rm.results, nil
}
