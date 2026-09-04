package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/juan7732/ergo/internal/workspace"
)

// searchListRows caps how many hits are visible at once; bubbles paginates
// beyond that.
const searchListRows = 12

// searchItem adapts a workspace.Hit to the bubbles list.
type searchItem struct {
	hit workspace.Hit
}

// FilterValue is the text the live filter matches against: the same fields
// `ergo search <query>` searches (name, URL, workspace), so anything the CLI
// finds can also be found by typing it here.
func (i searchItem) FilterValue() string {
	return i.hit.Name + " " + i.hit.URL + " " + i.hit.Workspace
}

// searchDelegate renders one hit per line, truncated to the list width.
type searchDelegate struct{}

func (searchDelegate) Height() int                             { return 1 }
func (searchDelegate) Spacing() int                            { return 0 }
func (searchDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (searchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(searchItem)
	if !ok {
		return
	}
	line := HitLine(it.hit)
	// A zero width means the terminal size is unknown (some ptys report
	// 0x0); render untruncated rather than blank.
	if w := m.Width() - 2; w > 0 {
		line = ansi.Truncate(line, w, "…")
	}
	if index == m.Index() {
		fmt.Fprint(w, StyleSelected.Render("> "+line))
		return
	}
	fmt.Fprint(w, "  "+line)
}

// HitLine is the compact one-line form of a hit used by the picker:
// name, kind, workspace, and the kind-specific on-disk state.
func HitLine(h workspace.Hit) string {
	return fmt.Sprintf("%s  %s  %s  %s", h.Name, string(h.Kind), h.Workspace, HitStateLabel(h))
}

// HitStateLabel renders a hit's kind-specific on-disk state, styled so
// present and absent are distinguishable at a glance. Shared by the search
// table and the picker so the vocabulary cannot drift.
func HitStateLabel(h workspace.Hit) string {
	var present, absent string
	switch h.Kind {
	case workspace.HitKindRepo:
		present, absent = "cloned", "uncloned"
	case workspace.HitKindFolder:
		present, absent = "created", "not created"
	default:
		present, absent = "synced", "not synced"
	}
	if h.Exists {
		return StyleSuccess.Render(present)
	}
	return StyleSubtle.Render(absent)
}

// SearchSelect is a Bubble Tea model that presents the full search index as
// a live-filtered list: typing narrows it, Enter picks the highlighted hit.
// It is the tenet-1 fallback for `ergo search` with no query.
type SearchSelect struct {
	list    list.Model
	initCmd tea.Cmd

	selected  workspace.Hit
	chosen    bool
	cancelled bool
}

// NewSearchSelect creates a SearchSelect over hits, which should be the full
// index (workspace.Search with an empty query).
func NewSearchSelect(hits []workspace.Hit) SearchSelect {
	items := make([]list.Item, len(hits))
	for i, h := range hits {
		items[i] = searchItem{hit: h}
	}

	// Height covers the filter line, the rows, and the pagination line. The
	// list has no title, status bar, or built-in help: the filter input is
	// the title row and View renders the help bar in the house style.
	l := list.New(nil, searchDelegate{}, 80, searchListRows+2)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)
	l.DisableQuitKeybindings()
	l.FilterInput.Prompt = "search: "
	l.FilterInput.PromptStyle = StylePrompt
	l.FilterInput.Placeholder = "type to filter…"
	l.SetSize(80, searchListRows+2)

	// DECISION: the list uses bubbles' default fuzzy filter even though the
	// CLI query is a plain substring match. Fuzzy is friendlier while typing
	// and the corpus (name + URL + workspace) is identical, so nothing the
	// CLI finds is unreachable here. Known consequence: URLs supply most
	// letters, so a two- or three-character query rarely shrinks the list
	// much; the filter ranks by score, so the closest match is still first
	// and under the cursor. Revisit (substring FilterFunc, or dropping the
	// URL from FilterValue) only if that proves confusing in practice.
	//
	// The list starts in its filtering state so the first keystroke narrows
	// it (fzf-style) rather than requiring "/" first. In that state the
	// visible set is only populated by the FilterMatchesMsg that SetItems
	// schedules, so that command is returned from Init.
	l.SetFilterState(list.Filtering)
	initCmd := l.SetItems(items)

	return SearchSelect{list: l, initCmd: initCmd}
}

func (m SearchSelect) Init() tea.Cmd {
	return tea.Batch(m.initCmd, textinput.Blink)
}

func (m SearchSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.list.SetWidth(msg.Width)
		}
		return m, nil

	case list.FilterMatchesMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		// New matches: highlight the best one and recompute pagination for
		// the new visible count (SetSize is the exported way to do that).
		m.list.GoToStart()
		m.list.SetSize(m.list.Width(), m.list.Height())
		return m, cmd

	case tea.KeyMsg:
		// Handled before the list sees them: while filtering, the list would
		// treat Esc as "clear filter" and Enter as "apply filter", and would
		// leave arrow keys to the text input.
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if it, ok := m.list.SelectedItem().(searchItem); ok {
				m.selected = it.hit
				m.chosen = true
				return m, tea.Quit
			}
			return m, nil

		case tea.KeyUp, tea.KeyCtrlP:
			m.list.CursorUp()
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN:
			m.list.CursorDown()
			return m, nil
		}
		// DECISION: "q" is not a cancel key here, unlike other pickers.
		// Every printable character feeds the filter (repos named "sqlite"
		// or "mq-client" must be typeable); Esc and Ctrl-C cancel.
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m SearchSelect) View() string {
	if m.cancelled || m.chosen {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.list.View() + "\n")
	b.WriteString(KeybindingHint("↑↓", "navigate") + "  " + KeybindingHint("enter", "print path") + "  " + KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the chosen hit and whether a selection was made. It is
// false after cancel and when Enter was never pressed.
func (m SearchSelect) Result() (workspace.Hit, bool) {
	if m.cancelled || !m.chosen {
		return workspace.Hit{}, false
	}
	return m.selected, true
}
