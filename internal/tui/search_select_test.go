package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/workspace"
)

func searchFixtureHits() []workspace.Hit {
	return []workspace.Hit{
		{Workspace: "platform", Kind: workspace.HitKindRepo, Name: "ergo",
			URL: "https://github.com/juan7732/ergo.git", Path: "/w/platform/ergo", Exists: true},
		{Workspace: "platform", Kind: workspace.HitKindRepo, Name: "billing",
			URL: "https://github.com/acme/billing.git", Path: "/w/platform/billing"},
		{Workspace: "scratch", Kind: workspace.HitKindFolder, Name: "notes",
			Path: "/w/scratch/notes"},
		{Workspace: "scratch", Kind: workspace.HitKindWorkspace, Name: "scratch",
			Path: "/w/scratch", Exists: true},
	}
}

func newSearchTest(t *testing.T) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, NewSearchSelect(searchFixtureHits()), teatest.WithInitialTermSize(80, 24))
}

// waitForOutput blocks until the accumulated render stream contains s.
func waitForOutput(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}, teatest.WithDuration(3*time.Second))
}

func finalSearchModel(t *testing.T, tm *teatest.TestModel) SearchSelect {
	t.Helper()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	m, ok := fm.(SearchSelect)
	require.True(t, ok, "final model is %T", fm)
	return m
}

func TestSearchSelect_EnterYieldsFirstHitWithoutFilter(t *testing.T) {
	tm := newSearchTest(t)

	// The visible set is populated asynchronously by Init's filter command;
	// wait until the first hit is rendered as the cursor row.
	waitForOutput(t, tm, "> ergo")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	hit, ok := finalSearchModel(t, tm).Result()
	require.True(t, ok)
	assert.Equal(t, "/w/platform/ergo", hit.Path)
}

func TestSearchSelect_TypingNarrowsThenEnterYieldsHit(t *testing.T) {
	tm := newSearchTest(t)
	waitForOutput(t, tm, "> ergo")

	// "acme" only appears in billing's URL: the filter covers URLs, like
	// the CLI query does.
	tm.Type("acme")
	waitForOutput(t, tm, "> billing")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	hit, ok := finalSearchModel(t, tm).Result()
	require.True(t, ok)
	assert.Equal(t, "billing", hit.Name)
	assert.Equal(t, "/w/platform/billing", hit.Path)
}

func TestSearchSelect_ArrowKeysMoveCursor(t *testing.T) {
	tm := newSearchTest(t)
	waitForOutput(t, tm, "> ergo")

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitForOutput(t, tm, "> billing")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	hit, ok := finalSearchModel(t, tm).Result()
	require.True(t, ok)
	assert.Equal(t, "billing", hit.Name)
}

func TestSearchSelect_EscCancels(t *testing.T) {
	tm := newSearchTest(t)
	waitForOutput(t, tm, "> ergo")

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	m := finalSearchModel(t, tm)
	_, ok := m.Result()
	assert.False(t, ok)
	assert.Equal(t, "", m.View(), "nothing is left on screen after cancel")
}

func TestSearchSelect_CtrlCCancels(t *testing.T) {
	tm := newSearchTest(t)
	waitForOutput(t, tm, "> ergo")

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	_, ok := finalSearchModel(t, tm).Result()
	assert.False(t, ok)
}

// TestSearchSelect_FilterNarrowsVisibleItems drives the model synchronously
// so the narrowed set itself can be inspected, not just the selection.
func TestSearchSelect_FilterNarrowsVisibleItems(t *testing.T) {
	m := NewSearchSelect(searchFixtureHits())

	// Run Init's filter command by hand to populate the visible set.
	var model tea.Model = m
	model, _ = model.Update(m.initCmd())
	require.Len(t, model.(SearchSelect).list.VisibleItems(), 4)

	// Type "acme", which only billing's URL contains: the list schedules a
	// filter command per keystroke; feed the last result back.
	var cmd tea.Cmd
	for _, r := range "acme" {
		model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	require.NotNil(t, cmd)
	model, _ = model.Update(drainForFilterMatches(t, cmd))

	visible := model.(SearchSelect).list.VisibleItems()
	require.Len(t, visible, 1, "only the repo whose URL contains acme matches")
	assert.Equal(t, "billing", visible[0].(searchItem).hit.Name)
}

// TestSearchSelect_FuzzyRanksBestMatchFirst pins the accepted fuzzy
// behavior: a short query such as "scr" also matches rows whose URL happens
// to contain s, c, r in order, so the set may not shrink much, but the
// closest match is ranked first and is what Enter would pick.
func TestSearchSelect_FuzzyRanksBestMatchFirst(t *testing.T) {
	m := NewSearchSelect(searchFixtureHits())
	var model tea.Model = m
	model, _ = model.Update(m.initCmd())

	var cmd tea.Cmd
	for _, r := range "scr" {
		model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(drainForFilterMatches(t, cmd))

	visible := model.(SearchSelect).list.VisibleItems()
	require.NotEmpty(t, visible)
	assert.Equal(t, "scratch", visible[0].(searchItem).hit.Workspace, "a scratch entry ranks first")
	sel, ok := model.(SearchSelect).list.SelectedItem().(searchItem)
	require.True(t, ok)
	assert.Equal(t, "scratch", sel.hit.Workspace, "the cursor sits on the best match")
}

// drainForFilterMatches executes cmd (possibly a batch) and returns the
// FilterMatchesMsg it produces.
func drainForFilterMatches(t *testing.T, cmd tea.Cmd) list.FilterMatchesMsg {
	t.Helper()
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if fm, ok := c().(list.FilterMatchesMsg); ok {
				return fm
			}
		}
		t.Fatal("batch contained no FilterMatchesMsg")
	}
	fm, ok := msg.(list.FilterMatchesMsg)
	require.True(t, ok, "expected FilterMatchesMsg, got %T", msg)
	return fm
}

func TestHitStateLabel_Vocabulary(t *testing.T) {
	tests := []struct {
		hit  workspace.Hit
		want string
	}{
		{workspace.Hit{Kind: workspace.HitKindRepo, Exists: true}, "cloned"},
		{workspace.Hit{Kind: workspace.HitKindRepo}, "uncloned"},
		{workspace.Hit{Kind: workspace.HitKindFolder, Exists: true}, "created"},
		{workspace.Hit{Kind: workspace.HitKindFolder}, "not created"},
		{workspace.Hit{Kind: workspace.HitKindWorkspace, Exists: true}, "synced"},
		{workspace.Hit{Kind: workspace.HitKindWorkspace}, "not synced"},
	}
	for _, tt := range tests {
		assert.Contains(t, HitStateLabel(tt.hit), tt.want)
	}
}
