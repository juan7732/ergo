package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// GroupSelectItem represents a group or tag choice in the group/tag selector.
type GroupSelectItem struct {
	// Name is the group or tag value.
	Name string
	// IsTag is true when this item is a tag; false when it is a group.
	IsTag bool
}

// GroupSelectResult holds the filter selection made in GroupSelect.
// Either Group is set (single group filter) or Tags is non-empty (tag filter).
type GroupSelectResult struct {
	// Group is the selected group name, or "" when no group was chosen.
	Group string
	// Tags is the list of selected tags, or nil when no tags were chosen.
	Tags []string
}

// GroupSelect is a Bubble Tea model for selecting a group or tags for ergo show.
// Groups are listed first, then tags. The user may select any combination;
// if any groups are selected the first selected group is used as the filter.
type GroupSelect struct {
	items    []GroupSelectItem
	selected []bool
	cursor   int

	confirmed bool
	cancelled bool
}

// NewGroupSelect creates a GroupSelect model from a list of items.
// List groups before tags for the intended section ordering.
func NewGroupSelect(items []GroupSelectItem) GroupSelect {
	return GroupSelect{
		items:    items,
		selected: make([]bool, len(items)),
	}
}

func (m GroupSelect) Init() tea.Cmd { return nil }

func (m GroupSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m GroupSelect) View() string {
	if m.cancelled || m.confirmed {
		return ""
	}

	var b strings.Builder
	b.WriteString(StyleTitle.Render("ergo show") + "\n\n")
	b.WriteString(StyleSubtle.Render("Select a group or tags to filter the workspace view:") + "\n\n")

	inTagSection := false
	for i, item := range m.items {
		// Print section headers on transition.
		if !item.IsTag && i == 0 {
			b.WriteString(StyleLabel.Render("  Groups") + "\n")
		} else if item.IsTag && !inTagSection {
			inTagSection = true
			b.WriteString("\n" + StyleLabel.Render("  Tags") + "\n")
		}

		check := StyleSubtle.Render("[ ]")
		if m.selected[i] {
			check = StyleSuccess.Render("[•]")
		}
		cursor := "  "
		if i == m.cursor {
			cursor = StyleSelected.Render("→ ")
		}
		b.WriteString(cursor + check + " " + item.Name + "\n")
	}

	b.WriteString("\n" +
		KeybindingHint("↑↓", "move") + "  " +
		KeybindingHint("space", "toggle") + "  " +
		KeybindingHint("enter", "confirm") + "  " +
		KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the GroupSelectResult and whether the user confirmed a selection.
// Returns (GroupSelectResult{}, false) if cancelled or nothing was selected.
func (m GroupSelect) Result() (GroupSelectResult, bool) {
	if !m.confirmed {
		return GroupSelectResult{}, false
	}

	var groups, tags []string
	for i, item := range m.items {
		if !m.selected[i] {
			continue
		}
		if item.IsTag {
			tags = append(tags, item.Name)
		} else {
			groups = append(groups, item.Name)
		}
	}

	if len(groups) == 0 && len(tags) == 0 {
		return GroupSelectResult{}, false
	}

	// DECISION: group takes precedence when both groups and tags are selected.
	// Use the first selected group to match the positional arg grammar of ergo show.
	if len(groups) > 0 {
		return GroupSelectResult{Group: groups[0]}, true
	}
	return GroupSelectResult{Tags: tags}, true
}
