//go:build integration

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// seedMaterializedWorkspace writes a TOML containing one fixture repo and runs
// `ergo open` so the workspace dir + .code-workspace file exist. Returns the
// workspace dir, suitable for use as a CWD for follow-on commands.
func seedMaterializedWorkspace(t *testing.T, h *harness.Harness, name string) string {
	t.Helper()
	h.InstallCodeStub()

	repo := h.SeedBareRepo(name+"-base", map[string]string{"x.txt": "1\n"})
	h.WriteWorkspaceTOML(name, fmt.Sprintf(`
[workspace]
name = %q

[[repos]]
url = %q
`, name, repo))

	h.Run("open", name).AssertOK(t)
	return h.WorkspaceDir(name)
}

func TestAddRepo_NonInteractive(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedMaterializedWorkspace(t, h, "ws-add-repo")

	res := h.RunIn(wsDir, "add", "repo",
		"https://example.com/owner/new-repo.git",
		"--tags=go,tools",
		"--group=tools",
	)
	res.AssertOK(t)

	body := h.ReadWorkspaceTOML("ws-add-repo")
	assert.Contains(t, body, "https://example.com/owner/new-repo.git")
	assert.Contains(t, body, "tools")
	assert.Contains(t, body, "go")

	h.Run("validate", "ws-add-repo").AssertOK(t)
}

func TestAddFolder_NonInteractive(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedMaterializedWorkspace(t, h, "ws-add-folder")

	res := h.RunIn(wsDir, "add", "folder", "scratch", "--git")
	res.AssertOK(t)

	body := h.ReadWorkspaceTOML("ws-add-folder")
	assert.Contains(t, body, `name = "scratch"`)
	assert.Contains(t, body, "git = true")
}

// TestAddRepo_PipedStdinAddsSilently pins the non-interactive contract of
// the shorthand: with stdin a pipe or closed, the add succeeds, exits 0, and
// never prints the sync prompt. The flag-absent default must keep this.
func TestAddRepo_PipedStdinAddsSilently(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedMaterializedWorkspace(t, h, "ws-add-quiet")
	repo := h.SeedBareRepo("quiet", map[string]string{"q.txt": "1\n"})

	// Closed stdin (/dev/null).
	res := h.RunIn(wsDir, "add", "repo", repo, "--name=quiet")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, `added repo "quiet"`)
	assert.NotContains(t, res.Combined, "sync workspace now?")
	assert.False(t, harness.IsGitRepo(filepath.Join(wsDir, "quiet")), "flag absent + no terminal: no sync")

	// Piped stdin with data on it: still no prompt, and the data is not
	// mistaken for an answer.
	res = h.RunWith(harness.RunOpts{Cwd: wsDir, Stdin: "y\n"}, "add", "folder", "piped-notes")
	res.AssertOK(t)
	assert.NotContains(t, res.Combined, "sync workspace now?")
	assert.NoDirExists(t, filepath.Join(wsDir, "piped-notes"))
}

func TestAddRepo_SyncFlagClonesInOneStep(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedMaterializedWorkspace(t, h, "ws-add-sync")
	repo := h.SeedBareRepo("one-step", map[string]string{"o.txt": "1\n"})

	res := h.RunIn(wsDir, "add", "repo", repo, "--group=core", "--sync")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, `added repo "one-step"`)
	assert.NotContains(t, res.Combined, "sync workspace now?", "--sync never prompts")
	assert.True(t, harness.IsGitRepo(filepath.Join(wsDir, "one-step")), "--sync clones the new repo")
	// The sync ran unfiltered: the pre-existing base repo is untouched and
	// still present, and the add's --group did not narrow anything.
	assert.True(t, harness.IsGitRepo(filepath.Join(wsDir, "ws-add-sync-base")))

	res = h.RunIn(wsDir, "add", "folder", "notes", "--sync")
	res.AssertOK(t)
	assert.DirExists(t, filepath.Join(wsDir, "notes"), "--sync creates the new folder")
}

func TestAddRepo_SyncFalseLeavesUncloned(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedMaterializedWorkspace(t, h, "ws-add-nosync")
	repo := h.SeedBareRepo("later", map[string]string{"l.txt": "1\n"})

	res := h.RunIn(wsDir, "add", "repo", repo, "--sync=false")
	res.AssertOK(t)
	assert.Contains(t, h.ReadWorkspaceTOML("ws-add-nosync"), repo, "the TOML entry is written")
	assert.NotContains(t, res.Combined, "sync workspace now?")
	assert.NoDirExists(t, filepath.Join(wsDir, "later"), "--sync=false never syncs")
}

func TestRemoveRepo_TOMLOnly(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	keep := h.SeedBareRepo("keep-r", map[string]string{"x": "1\n"})
	gone := h.SeedBareRepo("gone-r", map[string]string{"x": "1\n"})

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
	require.DirExists(t, filepath.Join(wsDir, "gone-r"))

	res := h.RunIn(wsDir, "remove", "repo", "gone-r")
	res.AssertOK(t)

	body := h.ReadWorkspaceTOML("ws")
	assert.NotContains(t, body, "/gone-r.git")
	assert.DirExists(t, filepath.Join(wsDir, "gone-r"), "remove without --force must not delete from disk")
}

func TestRemoveRepo_ForceDeletesDisk(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	repo := h.SeedBareRepo("doomed", map[string]string{"x": "1\n"})
	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
`, repo))

	h.Run("open", "ws").AssertOK(t)
	wsDir := h.WorkspaceDir("ws")
	require.DirExists(t, filepath.Join(wsDir, "doomed"))

	res := h.RunWith(harness.RunOpts{
		Cwd:   wsDir,
		Stdin: "y\n",
	}, "remove", "repo", "doomed", "--force")
	res.AssertOK(t)

	assert.NoDirExists(t, filepath.Join(wsDir, "doomed"))
}

func TestEdit_OpensTOMLInVSCode(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	h.WriteWorkspaceTOML("ws", `
[workspace]
name = "ws"
`)

	res := h.Run("edit", "ws")
	res.AssertOK(t)

	invs := h.CodeInvocations()
	require.Len(t, invs, 1)
	assert.Equal(t, h.WorkspaceTOMLPath("ws"), invs[0][0])
}

func TestEdit_GlobalFlagOpensGlobalConfig(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	h.WriteGlobalConfig(`
[defaults]
workspace_root = "~/ergo-workspaces"
default_branch = "main"
`)

	res := h.Run("edit", "--global")
	res.AssertOK(t)

	invs := h.CodeInvocations()
	require.Len(t, invs, 1)
	expectedPath := filepath.Join(h.Home, ".ergo", "config.toml")
	assert.Equal(t, expectedPath, invs[0][0])
}

func TestEdit_GlobalFlagRejectsWorkspaceArg(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	res := h.Run("edit", "--global", "ws")
	res.AssertFail(t)
	assert.Contains(t, res.Stderr, "--global does not take a workspace name")
}
