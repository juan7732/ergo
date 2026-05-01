package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// RemoveItem represents a selectable repo or folder entry in the remove selector.
type RemoveItem struct {
	Name   string
	IsRepo bool
}

// RemoveSelect is a Bubble Tea model for multi-selecting repos/folders to remove.
type RemoveSelect struct {
	items    []RemoveItem
	selected []bool
	cursor   int

	confirmed bool
	cancelled bool
}

// NewRemoveSelect creates a RemoveSelect from a list of workspace items.
func NewRemoveSelect(items []RemoveItem) RemoveSelect {
	return RemoveSelect{
		items:    items,
		selected: make([]bool, len(items)),
	}
}

func (m RemoveSelect) Init() tea.Cmd { return nil }

func (m RemoveSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.confirmed = true
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case tea.KeyRunes:
			if msg.String() == " " {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		}
	}
	return m, nil
}

func (m RemoveSelect) View() string {
	if m.cancelled || m.confirmed {
		return ""
	}

	var b strings.Builder
	b.WriteString(StyleTitle.Render("ergo remove") + "\n\n")
	b.WriteString(StyleSubtle.Render("Select items to remove:") + "\n\n")

	for i, item := range m.items {
		check := StyleSubtle.Render("[ ]")
		if m.selected[i] {
			check = StyleError.Render("[x]")
		}
		kind := "folder"
		if item.IsRepo {
			kind = "repo"
		}
		cursor := "  "
		if i == m.cursor {
			cursor = StyleSelected.Render("→ ")
		}
		b.WriteString(cursor + check + " " + item.Name + "  " + StyleSubtle.Render(kind) + "\n")
	}

	b.WriteString("\n" + KeybindingHint("↑↓", "move") + "  " + KeybindingHint("space", "toggle") + "  " + KeybindingHint("enter", "confirm") + "  " + KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the selected items and whether the user confirmed.
// Returns (nil, false) if the user cancelled or no items were selected.
func (m RemoveSelect) Result() ([]RemoveItem, bool) {
	if !m.confirmed {
		return nil, false
	}
	var selected []RemoveItem
	for i, item := range m.items {
		if m.selected[i] {
			selected = append(selected, item)
		}
	}
	return selected, true
}
