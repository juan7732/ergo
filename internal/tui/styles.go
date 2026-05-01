package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Accent is the primary highlight color.
	Accent = lipgloss.Color("212")

	// Subtle is used for secondary/muted text.
	Subtle = lipgloss.Color("241")

	// Error is used for error messages.
	Error = lipgloss.Color("196")

	// Success is used for success messages.
	Success = lipgloss.Color("82")

	// Warning is used for warning messages.
	Warning = lipgloss.Color("214")

	// StyleTitle renders a command/section title.
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Accent)

	// StyleSubtle renders helper text and hints.
	StyleSubtle = lipgloss.NewStyle().
			Foreground(Subtle)

	// StyleError renders error text.
	StyleError = lipgloss.NewStyle().
			Foreground(Error)

	// StyleSuccess renders success text.
	StyleSuccess = lipgloss.NewStyle().
			Foreground(Success)

	// StyleWarning renders warning text.
	StyleWarning = lipgloss.NewStyle().
			Foreground(Warning)

	// StyleLabel renders a field label.
	StyleLabel = lipgloss.NewStyle().
			Bold(true)

	// StylePrompt is the cursor/prompt character style.
	StylePrompt = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// StyleKeybinding renders a key hint at the bottom of a view.
	StyleKeybinding = lipgloss.NewStyle().
			Foreground(Subtle)

	// StyleSelected renders a selected item in a list.
	StyleSelected = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// StyleTableHeader renders a table column header.
	StyleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Underline(true)

	// StyleTableBorder is the color for table border characters.
	StyleTableBorder = lipgloss.NewStyle().
				Foreground(Subtle)
)

// KeybindingHint formats a key + description pair for the help bar.
// e.g. KeybindingHint("enter", "confirm") → "enter confirm"
func KeybindingHint(key, desc string) string {
	return StylePrompt.Render(key) + StyleKeybinding.Render(" "+desc)
}
