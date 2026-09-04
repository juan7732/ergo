package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/workspace"
)

// withSearchSeams swaps the stdin gate and the picker for the test and
// captures the command's stdout and stderr.
func withSearchSeams(t *testing.T, terminal bool, picker func([]workspace.Hit) (workspace.Hit, bool, error)) (stdout, stderr *bytes.Buffer) {
	t.Helper()
	origTerm, origPicker := stdinIsTerminal, searchPicker
	origSilence := searchCmd.SilenceErrors
	t.Cleanup(func() {
		stdinIsTerminal, searchPicker = origTerm, origPicker
		searchCmd.SilenceErrors = origSilence
		searchCmd.SetOut(nil)
		searchCmd.SetErr(nil)
	})
	stdinIsTerminal = func() bool { return terminal }
	if picker != nil {
		searchPicker = picker
	}
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	searchCmd.SetOut(stdout)
	searchCmd.SetErr(stderr)
	return stdout, stderr
}

// seedSearchHome points HOME at a temp dir holding one workspace TOML so
// runSearch has an index to load.
func seedSearchHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".ergo", "workspaces")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ws.toml"),
		[]byte("[workspace]\nname = \"ws\"\n\n[[repos]]\nurl = \"https://example.com/ergo.git\"\n"), 0o600))
}

// TestRunSearch_NoArgsWithoutTerminalFailsFast is the guard for the flagship
// wrapper `d=$(ergo search) && cd "$d"`: the gate must look at stdin only,
// and a piped stdin must produce a usage error before any config is read or
// anything is rendered.
func TestRunSearch_NoArgsWithoutTerminalFailsFast(t *testing.T) {
	pickerCalled := false
	stdout, _ := withSearchSeams(t, false, func([]workspace.Hit) (workspace.Hit, bool, error) {
		pickerCalled = true
		return workspace.Hit{}, false, nil
	})
	// No HOME seeding on purpose: the gate must fire before config loads.
	t.Setenv("HOME", filepath.Join(t.TempDir(), "never-created"))

	err := runSearch(searchCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: ergo search <query>")
	assert.Contains(t, err.Error(), "terminal on stdin")
	assert.False(t, pickerCalled, "the picker must never start without a terminal")
	assert.Empty(t, stdout.String(), "nothing may reach stdout")
}

func TestRunSearch_PickerSelectionPrintsOnlyPath(t *testing.T) {
	seedSearchHome(t)
	var received []workspace.Hit
	stdout, stderr := withSearchSeams(t, true, func(hits []workspace.Hit) (workspace.Hit, bool, error) {
		received = hits
		return workspace.Hit{Name: "ergo", Kind: workspace.HitKindRepo, Path: "/w/ws/ergo"}, true, nil
	})

	require.NoError(t, runSearch(searchCmd, nil))

	assert.Equal(t, "/w/ws/ergo\n", stdout.String(), "stdout carries exactly the path line")
	assert.Empty(t, stderr.String())
	// The picker receives the full index: the workspace itself and its repo.
	require.Len(t, received, 2)
	assert.Equal(t, workspace.HitKindWorkspace, received[0].Kind)
	assert.Equal(t, "ergo", received[1].Name)
}

func TestRunSearch_PickerCancelExitsNonZeroWithCleanStdout(t *testing.T) {
	seedSearchHome(t)
	stdout, stderr := withSearchSeams(t, true, func([]workspace.Hit) (workspace.Hit, bool, error) {
		return workspace.Hit{}, false, nil
	})

	err := runSearch(searchCmd, nil)
	require.ErrorIs(t, err, errSearchCancelled)
	assert.Empty(t, stdout.String(), "cancel must leave stdout empty")
	assert.Equal(t, "cancelled\n", stderr.String())
	assert.True(t, searchCmd.SilenceErrors, "cobra must not add an Error: line for a cancel")
}

func TestRunSearch_JSONWithoutQueryIsFullIndexNotPicker(t *testing.T) {
	seedSearchHome(t)
	pickerCalled := false
	stdout, _ := withSearchSeams(t, false, func([]workspace.Hit) (workspace.Hit, bool, error) {
		pickerCalled = true
		return workspace.Hit{}, false, nil
	})
	require.NoError(t, searchCmd.Flags().Set("json", "true"))
	t.Cleanup(func() { _ = searchCmd.Flags().Set("json", "false") })

	require.NoError(t, runSearch(searchCmd, nil))

	assert.False(t, pickerCalled)
	assert.Contains(t, stdout.String(), `"query": ""`)
	assert.Contains(t, stdout.String(), `"name": "ergo"`)
}

func TestRunSearch_BlankQueryStillRejected(t *testing.T) {
	stdout, _ := withSearchSeams(t, true, nil)
	err := runSearch(searchCmd, []string{"   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
	assert.Empty(t, stdout.String())
}
