package tui

import "fmt"

// GitUIAdapter satisfies the git.UI interface using TUI styles.
// It is defined here to avoid circular imports — the git package
// defines the interface, and this adapter wires in the TUI implementation.
type GitUIAdapter struct{}

// Confirm asks a yes/no question using the TUI prompt.
func (GitUIAdapter) Confirm(prompt string) (bool, error) {
	return Confirm(prompt)
}

// PrintSuccess prints a success-styled message.
func (GitUIAdapter) PrintSuccess(msg string) {
	fmt.Printf("%s %s\n", SuccessStyle.Render(IconCheck), msg)
}

// PrintDim prints a dim/muted message.
func (GitUIAdapter) PrintDim(msg string) {
	fmt.Println(DimStyle.Render(msg))
}
