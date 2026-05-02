//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"juan7732/ergo/test/integration/harness"
)

// TestOpen_FirstRunMaterializesAndLaunches verifies that `ergo open` on a
// fresh workspace clones every repo, generates a valid .code-workspace with
// the mandatory root entry, and invokes the `code` shim.
func TestOpen_FirstRunMaterializesAndLaunches(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	repoA := h.SeedBareRepo("alpha", map[string]string{"main.go": "package main\n"})
	repoB := h.SeedBareRepo("beta", map[string]string{"README.md": "# beta\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q

[[repos]]
url = %q

[[folders]]
name = "scratch"
`, repoA, repoB))

	res := h.Run("open", "ws")
	res.AssertOK(t)

	wsDir := h.WorkspaceDir("ws")
	assert.DirExists(t, filepath.Join(wsDir, "alpha"))
	assert.DirExists(t, filepath.Join(wsDir, "beta"))
	assert.DirExists(t, filepath.Join(wsDir, "scratch"))
	assert.True(t, harness.IsGitRepo(filepath.Join(wsDir, "alpha")))

	// .code-workspace must include the root entry first plus all folders.
	raw := h.ReadCodeWorkspace("ws")
	var parsed struct {
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
		Ergo map[string]any `json:"ergo"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))

	require.NotEmpty(t, parsed.Folders, "expected at least the root folder")
	assert.Equal(t, "root", parsed.Folders[0].Name)
	assert.Equal(t, ".", parsed.Folders[0].Path)

	// Workspace-name metadata in the ergo object.
	assert.Equal(t, "ws", parsed.Ergo["workspace-name"])

	// `code` should have been invoked exactly once with the .code-workspace path.
	invs := h.CodeInvocations()
	require.Len(t, invs, 1, "expected exactly one code invocation")
	assert.Equal(t, h.CodeWorkspaceFile("ws"), invs[0][0])
}

// TestOpen_SecondRunIsNoOp asserts smart regeneration: rerunning open doesn't
// rewrite the .code-workspace file when content is unchanged.
func TestOpen_SecondRunIsNoOp(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	repo := h.SeedBareRepo("only", map[string]string{"x.txt": "hi\n"})
	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
`, repo))

	h.Run("open", "ws").AssertOK(t)

	info1, err := os.Stat(h.CodeWorkspaceFile("ws"))
	require.NoError(t, err)
	firstMtime := info1.ModTime()

	// Second run: smart regen should not touch the file.
	h.Run("open", "ws").AssertOK(t)

	info2, err := os.Stat(h.CodeWorkspaceFile("ws"))
	require.NoError(t, err)
	assert.True(t, info2.ModTime().Equal(firstMtime),
		"smart regen should not rewrite .code-workspace; mtime changed from %v to %v",
		firstMtime, info2.ModTime())
}

// TestSync_ForceDeletesOrphans removes a repo from the TOML after materialization
// and checks that `sync --force` (with confirmation piped on stdin) deletes the
// orphaned directory.
func TestSync_ForceDeletesOrphans(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	keep := h.SeedBareRepo("keep", map[string]string{"a.txt": "a\n"})
	gone := h.SeedBareRepo("gone", map[string]string{"b.txt": "b\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q

[[repos]]
url = %q
`, keep, gone))

	h.Run("open", "ws").AssertOK(t)
	wsDir := h.WorkspaceDir("ws")
	assert.DirExists(t, filepath.Join(wsDir, "gone"))

	// Drop "gone" from the TOML and re-sync with --force.
	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
`, keep))

	res := h.RunWith(harness.RunOpts{Stdin: "y\n"}, "sync", "ws", "--force")
	res.AssertOK(t)

	assert.NoDirExists(t, filepath.Join(wsDir, "gone"), "gone/ should have been removed")
	assert.DirExists(t, filepath.Join(wsDir, "keep"))
}
