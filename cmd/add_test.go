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
