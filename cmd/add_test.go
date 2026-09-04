package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/workspace"
)

// TestPromptSync_DoesNotInheritCommandFilterFlags guards against the flag-leak
// bug: when `ergo add repo <url> --name=foo` prompts to sync, the follow-up
// sync must run unfiltered. Reusing the add command's context would leak
// --name/--group/--tags into the sync, silently restricting it (and, combined
// with --force, risking deletion of out-of-filter repos).
func TestPromptSync_DoesNotInheritCommandFilterFlags(t *testing.T) {
	// Force the interactive path and capture the sync invocation instead of
	// running a real sync (no git, no filesystem).
	origTerm := stdinIsTerminal
	origRunner := syncRunner
	t.Cleanup(func() {
		stdinIsTerminal = origTerm
		syncRunner = origRunner
	})
	stdinIsTerminal = func() bool { return true }

	var gotName string
	var gotParams syncParams
	var called bool
	syncRunner = func(_ *cobra.Command, name string, p syncParams) error {
		called = true
		gotName = name
		gotParams = p
		return nil
	}

	// Simulate the add command carrying filter-shaped flags that must NOT leak.
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "should-not-leak", "")
	cmd.Flags().String("group", "core", "")
	cmd.Flags().StringSlice("tags", []string{"go"}, "")
	cmd.SetIn(strings.NewReader("y\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, promptSync(cmd, "ws"))

	require.True(t, called, "sync should run after a 'y' confirmation")
	assert.Equal(t, "ws", gotName)
	assert.Equal(t, workspace.FilterOptions{}, gotParams.Filter,
		"sync triggered from add must use zero-value filter options")
	assert.False(t, gotParams.Force, "add-triggered sync must not force-delete")
	assert.False(t, gotParams.Add, "add-triggered sync must not adopt orphans")
}

// addSeams forces the stdin gate for the test and records sync invocations
// instead of running a real sync.
func addSeams(t *testing.T, terminal bool) (syncCalls *int, gotParams *syncParams) {
	t.Helper()
	origTerm, origRunner := stdinIsTerminal, syncRunner
	t.Cleanup(func() {
		stdinIsTerminal = origTerm
		syncRunner = origRunner
	})
	stdinIsTerminal = func() bool { return terminal }
	calls, params := 0, syncParams{}
	syncRunner = func(_ *cobra.Command, _ string, p syncParams) error {
		calls++
		params = p
		return nil
	}
	return &calls, &params
}

// newAddShorthandCmd mirrors the shorthand subcommands' flag set: the --sync
// tri-state plus an add-only flag that must never leak into the sync.
func newAddShorthandCmd(t *testing.T, stdin string, args ...string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("sync", false, "")
	cmd.Flags().String("name", "should-not-leak", "")
	require.NoError(t, cmd.Flags().Parse(args))
	cmd.SetIn(strings.NewReader(stdin))
	var out bytes.Buffer
	cmd.SetOut(&out)
	return cmd, &out
}

func TestAfterAdd_FlagAbsentNonTTY_NoPromptNoSync(t *testing.T) {
	calls, _ := addSeams(t, false)
	cmd, out := newAddShorthandCmd(t, "y\n")

	require.NoError(t, afterAdd(cmd, "ws"))

	assert.Equal(t, 0, *calls, "no sync without a terminal")
	assert.Empty(t, out.String(), "no prompt without a terminal")
}

func TestAfterAdd_FlagAbsentTTY_PromptsAndSyncsOnYes(t *testing.T) {
	calls, params := addSeams(t, true)
	cmd, out := newAddShorthandCmd(t, "y\n")

	require.NoError(t, afterAdd(cmd, "ws"))

	assert.Contains(t, out.String(), "sync workspace now? [y/N]")
	assert.Equal(t, 1, *calls)
	assert.Equal(t, syncParams{}, *params, "add-triggered sync uses zero-value params")
}

func TestAfterAdd_FlagAbsentTTY_NoSyncOnNo(t *testing.T) {
	calls, _ := addSeams(t, true)
	cmd, out := newAddShorthandCmd(t, "n\n")

	require.NoError(t, afterAdd(cmd, "ws"))

	assert.Contains(t, out.String(), "sync workspace now? [y/N]")
	assert.Equal(t, 0, *calls)
}

func TestAfterAdd_SyncFlag_SyncsWithoutPrompting(t *testing.T) {
	for _, terminal := range []bool{true, false} {
		calls, params := addSeams(t, terminal)
		// Stdin says "n": it must not be consulted at all.
		cmd, out := newAddShorthandCmd(t, "n\n", "--sync")

		require.NoError(t, afterAdd(cmd, "ws"))

		assert.Equal(t, 1, *calls, "terminal=%v: --sync runs the sync", terminal)
		assert.Equal(t, syncParams{}, *params, "add-only flags must not leak into the sync")
		assert.NotContains(t, out.String(), "sync workspace now?", "terminal=%v: --sync never prompts", terminal)
	}
}

func TestAfterAdd_SyncFalse_NeverPromptsNeverSyncs(t *testing.T) {
	calls, _ := addSeams(t, true)
	cmd, out := newAddShorthandCmd(t, "y\n", "--sync=false")

	require.NoError(t, afterAdd(cmd, "ws"))

	assert.Equal(t, 0, *calls)
	assert.Empty(t, out.String())
}
