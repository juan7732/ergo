package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// WorkspaceSelect is a Bubble Tea model that presents a filterable list of
// workspace names and lets the user pick one.
type WorkspaceSelect struct {
	all      []string // full list of workspace names
	filtered []string // names matching the current filter
	cursor   int

	input     textinput.Model
	selected  string
	cancelled bool
}

// NewWorkspaceSelect creates a WorkspaceSelect model.
// initial is the pre-filled filter string (e.g. from a partial name arg).
func NewWorkspaceSelect(names []string, initial string) WorkspaceSelect {
	ti := newInput(initial, "filter…", 64)
	m := WorkspaceSelect{
		all:   names,
		input: ti,
	}
	m.filtered = m.applyFilter(initial)
	return m
}

func (m WorkspaceSelect) Init() tea.Cmd {
	return textinput.Blink
}

func (m WorkspaceSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor]
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	// Update the text input and re-filter.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filtered = m.applyFilter(m.input.Value())
	// Keep cursor in bounds after filter changes.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	return m, cmd
}

func (m WorkspaceSelect) View() string {
	if m.cancelled || m.selected != "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(StyleTitle.Render("Select workspace") + "\n\n")
	b.WriteString(m.input.View() + "\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(StyleSubtle.Render("  no matches") + "\n")
	} else {
		for i, name := range m.filtered {
			if i == m.cursor {
				b.WriteString("  " + StyleSelected.Render("> "+name) + "\n")
			} else {
				b.WriteString("    " + name + "\n")
			}
		}
	}

	b.WriteString("\n" + KeybindingHint("↑↓", "navigate") + "  " + KeybindingHint("enter", "select") + "  " + KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the selected workspace name and whether a selection was made.
func (m WorkspaceSelect) Result() (string, bool) {
	if m.cancelled || m.selected == "" {
		return "", false
	}
	return m.selected, true
}

// applyFilter returns names that contain filter as a case-insensitive substring.
func (m WorkspaceSelect) applyFilter(filter string) []string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		out := make([]string, len(m.all))
		copy(out, m.all)
		return out
	}
	var out []string
	for _, name := range m.all {
		if strings.Contains(strings.ToLower(name), filter) {
			out = append(out, name)
		}
	}
	return out
}
