//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// seedFilteredWorkspace materializes a two-repo workspace (groups "ml" and
// "tools") and activates `show ml`, returning the bare tools repo URL and
// the workspace dir.
func seedFilteredWorkspace(t *testing.T, h *harness.Harness) (toolsBare, wsDir string) {
	t.Helper()

	mlRepo := h.SeedBareRepo("ml-a", map[string]string{"x.txt": "1\n"})
	toolsBare = h.SeedBareRepo("tools-b", map[string]string{"y.txt": "1\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
group = "ml"

[[repos]]
url = %q
group = "tools"
`, mlRepo, toolsBare))

	h.Run("open", "ws").AssertOK(t)
	wsDir = h.WorkspaceDir("ws")
	h.RunIn(wsDir, "show", "ml").AssertOK(t)

	ws := string(h.ReadCodeWorkspace("ws"))
	require.Contains(t, ws, `"group": "ml"`, "show must record the filter")
	require.NotContains(t, ws, `"name": "tools-b"`, "filtered repo must be out of the folders list")
	return toolsBare, wsDir
}

// TestSync_PreservesShowFilter covers ergo-vscode-spec.md §3.2: sync must
// re-apply an active show filter instead of silently resetting the view,
// while still operating on the full TOML (hidden repos are still pulled).
func TestSync_PreservesShowFilter(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	toolsBare, wsDir := seedFilteredWorkspace(t, h)

	// Push a commit upstream into the hidden repo so a pull has something to do.
	h.MutateBareRepo(toolsBare, func(work string) {
		require.NoError(t, os.WriteFile(filepath.Join(work, "new.txt"), []byte("upstream\n"), 0o644))
	})

	res := h.Run("sync", "ws")
	res.AssertOK(t)

	// The note line surfaces the preserved filter.
	assert.Contains(t, res.Combined, "note: show filter active")
	assert.Contains(t, res.Combined, `group "ml"`)
	assert.Contains(t, res.Combined, "1 of 2 repos visible")
	assert.Contains(t, res.Combined, "ergo show all")

	// The workspace file still carries the filter and only the filtered repo.
	ws := string(h.ReadCodeWorkspace("ws"))
	assert.Contains(t, ws, `"group": "ml"`, "sync must not clobber the recorded filter")
	assert.Contains(t, ws, `"name": "ml-a"`)
	assert.NotContains(t, ws, `"name": "tools-b"`)
	assert.Contains(t, ws, `"name": "root"`, "root folder is always present")

	// The hidden repo was still pulled: the filter is a view concern, not an
	// operation filter.
	assert.FileExists(t, filepath.Join(wsDir, "tools-b", "new.txt"),
		"repos hidden by the show filter must still be pulled by sync")
}

// TestOpen_PreservesShowFilter: open regenerates the workspace file without
// resetting an active filter, and its fast path treats the filtered file as
// current rather than stale.
func TestOpen_PreservesShowFilter(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	_, _ = seedFilteredWorkspace(t, h)

	res := h.Run("open", "ws")
	res.AssertOK(t)

	assert.Contains(t, res.Combined, "note: show filter active")
	assert.Contains(t, res.Combined, `group "ml"`)

	ws := string(h.ReadCodeWorkspace("ws"))
	assert.Contains(t, ws, `"group": "ml"`, "open must not clobber the recorded filter")
	assert.NotContains(t, ws, `"name": "tools-b"`)

	// --print-dir keeps stdout clean: the note goes to stderr.
	res = h.Run("open", "ws", "--print-dir")
	res.AssertOK(t)
	assert.Equal(t, h.WorkspaceDir("ws")+"\n", res.Stdout, "stdout must be the directory only")
	assert.Contains(t, res.Stderr, "note: show filter active")
}

// TestStatus_ShowsFilterNoteHeader: human-format status surfaces the active
// filter as a header line; --short and --json stay clean for machines.
func TestStatus_ShowsFilterNoteHeader(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	_, _ = seedFilteredWorkspace(t, h)

	res := h.Run("status", "ws")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "note: show filter active")
	assert.Contains(t, res.Stdout, `group "ml"`)

	res = h.Run("status", "ws", "--short")
	res.AssertOK(t)
	assert.NotContains(t, res.Stdout, "note:", "--short stays one-line-per-repo")

	res = h.Run("status", "ws", "--json")
	res.AssertOK(t)
	assert.NotContains(t, res.Stdout, "note:", "--json carries no prose; consumers read show --json")
}

// TestSync_MalformedWorkspaceFileFallsBack: a corrupt .code-workspace must
// never fail sync — it falls back to regenerating the full, unfiltered view.
func TestSync_MalformedWorkspaceFileFallsBack(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	_, _ = seedFilteredWorkspace(t, h)

	require.NoError(t, os.WriteFile(h.CodeWorkspaceFile("ws"), []byte("{ not json"), 0o600))

	res := h.Run("sync", "ws")
	res.AssertOK(t)
	assert.NotContains(t, res.Combined, "note: show filter active")

	ws := string(h.ReadCodeWorkspace("ws"))
	assert.NotContains(t, ws, `"filter"`, "fallback regenerates without a filter")
	assert.Contains(t, ws, `"name": "ml-a"`)
	assert.Contains(t, ws, `"name": "tools-b"`, "fallback restores the full folders list")
}
