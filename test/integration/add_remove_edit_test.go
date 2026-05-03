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
