package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner implements git.Runner for tests.
type fakeRunner struct {
	out string
	err error
}

func (f fakeRunner) Run(_, _ string, _ ...string) (string, error) {
	return f.out, f.err
}

// writeCodeWorkspace writes a minimal .code-workspace JSON file under dir with
// the given workspace name embedded in the "ergo" key.
func writeCodeWorkspace(t *testing.T, dir, filename, wsName string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"ergo": map[string]any{"workspace-name": wsName},
		"folders": []any{},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), data, 0o644))
}

func TestDetect_FindsCodeWorkspaceInSameDir(t *testing.T) {
	tmp := t.TempDir()
	writeCodeWorkspace(t, tmp, "my-ws.code-workspace", "my-ws")

	d, err := Detect(tmp, fakeRunner{})
	require.NoError(t, err)
	assert.Equal(t, "my-ws", d.WorkspaceName)
	assert.False(t, d.IsStandaloneRepo)
}

func TestDetect_FindsCodeWorkspaceInParent(t *testing.T) {
	tmp := t.TempDir()
	writeCodeWorkspace(t, tmp, "parent-ws.code-workspace", "parent-ws")
	nested := filepath.Join(tmp, "sub", "dir")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	d, err := Detect(nested, fakeRunner{})
	require.NoError(t, err)
	assert.Equal(t, "parent-ws", d.WorkspaceName)
}

func TestDetect_IgnoresCodeWorkspaceWithoutErgoKey(t *testing.T) {
	tmp := t.TempDir()
	// Write a plain VS Code workspace file without the "ergo" key.
	data := []byte(`{"folders": [{"path": "."}]}`)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "other.code-workspace"), data, 0o644))

	// Stand-alone git repo detection should be tried next; make it fail too.
	d, err := Detect(tmp, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Empty(t, d.WorkspaceName)
	assert.False(t, d.IsStandaloneRepo)
}

func TestDetect_IgnoresMalformedCodeWorkspace(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "bad.code-workspace"), []byte("not json {{{"), 0o644))

	d, err := Detect(tmp, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Empty(t, d.WorkspaceName)
}

func TestDetect_MatchesKnownRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the workspace TOML so ListWorkspaceNames returns "my-project".
	wsDir := filepath.Join(tmp, ".ergo", "workspaces")
	require.NoError(t, os.MkdirAll(wsDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "my-project.toml"), []byte(`
[workspace]
name = "my-project"
`), 0o600))

	// Set workspace_root in the global config.
	ergoDir := filepath.Join(tmp, ".ergo")
	require.NoError(t, os.WriteFile(filepath.Join(ergoDir, "config.toml"), []byte(`
[defaults]
workspace_root = "`+filepath.Join(tmp, "ergo-workspaces")+`"
default_branch = "main"
`), 0o600))

	// CWD is inside the workspace directory.
	cwd := filepath.Join(tmp, "ergo-workspaces", "my-project", "some-repo")
	require.NoError(t, os.MkdirAll(cwd, 0o755))

	d, err := Detect(cwd, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Equal(t, "my-project", d.WorkspaceName)
}

func TestDetect_StandaloneRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No .code-workspace, no known root match.
	// Git runner succeeds and reports a repo root.
	r := fakeRunner{out: filepath.Join(tmp, "my-repo")}

	cwd := filepath.Join(tmp, "my-repo", "pkg")
	require.NoError(t, os.MkdirAll(cwd, 0o755))

	d, err := Detect(cwd, r)
	require.NoError(t, err)
	assert.Empty(t, d.WorkspaceName)
	assert.True(t, d.IsStandaloneRepo)
	assert.Equal(t, filepath.Join(tmp, "my-repo"), d.StandaloneRepoRoot)
}

func TestDetect_NeitherWorkspaceNorRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d, err := Detect(tmp, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Empty(t, d.WorkspaceName)
	assert.False(t, d.IsStandaloneRepo)
}
