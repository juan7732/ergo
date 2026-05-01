package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts a Bubble Tea program with the given model and returns the final
// model after the program exits. It uses the standard full-window renderer.
func Run(m tea.Model) (tea.Model, error) {
	p := tea.NewProgram(m, tea.WithAltScreen())
	return p.Run()
}

// RunInline starts a Bubble Tea program in inline mode (no alt screen).
// Suitable for short flows that print output and return to the shell.
func RunInline(m tea.Model) (tea.Model, error) {
	p := tea.NewProgram(m)
	return p.Run()
}
