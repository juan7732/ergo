package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupWorkspaceNames writes stub TOML files for the given workspace names
// under a temp HOME directory and returns the temp dir. Sets HOME env var.
func setupWorkspaceNames(t *testing.T, names ...string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	wsDir := filepath.Join(tmp, ".ergo", "workspaces")
	require.NoError(t, os.MkdirAll(wsDir, 0o700))
	for _, name := range names {
		require.NoError(t, os.WriteFile(
			filepath.Join(wsDir, name+".toml"),
			[]byte("[workspace]\nname = \""+name+"\"\n"),
			0o600,
		))
	}
	// Write a minimal global config with workspace_root outside tmp so known-root
	// detection won't accidentally match.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, ".ergo", "config.toml"),
		[]byte("[defaults]\nworkspace_root = \"/nonexistent/ergo-workspaces\"\ndefault_branch = \"main\"\n"),
		0o600,
	))
	return tmp
}

func TestResolve_ExactMatch(t *testing.T) {
	setupWorkspaceNames(t, "ml-projects", "side-projects", "knowledge-base")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	result, err := Resolve("ml-projects", "/tmp", noRepo)
	require.NoError(t, err)
	assert.Equal(t, "ml-projects", result.Name)
	assert.Nil(t, result.Candidates)
}

func TestResolve_SinglePartialMatch_ResolvesDirectly(t *testing.T) {
	setupWorkspaceNames(t, "ml-projects", "side-projects", "knowledge-base")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	result, err := Resolve("ml-pro", "/tmp", noRepo)
	require.NoError(t, err)
	assert.Equal(t, "ml-projects", result.Name)
	assert.Nil(t, result.Candidates)
}

func TestResolve_MultiplePartialMatches_ReturnsCandidates(t *testing.T) {
	setupWorkspaceNames(t, "ml-projects", "ml-experiments", "side-projects")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	result, err := Resolve("ml", "/tmp", noRepo)
	require.NoError(t, err)
	assert.Empty(t, result.Name)
	assert.ElementsMatch(t, []string{"ml-projects", "ml-experiments"}, result.Candidates)
}

func TestResolve_NoMatch_ErrorWithSuggestion(t *testing.T) {
	setupWorkspaceNames(t, "ml-projects", "side-projects")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	_, err := Resolve("ml-projcts", "/tmp", noRepo) // typo
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ml-projects")
	assert.Contains(t, err.Error(), "did you mean")
}

func TestResolve_NoMatch_ErrorNoSuggestion(t *testing.T) {
	setupWorkspaceNames(t, "ml-projects", "side-projects")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	_, err := Resolve("zzz-totally-unknown", "/tmp", noRepo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zzz-totally-unknown")
	assert.NotContains(t, err.Error(), "did you mean")
}

func TestResolve_DotArg_CurrentWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write workspace TOML so we can seed the .code-workspace file.
	wsDir := filepath.Join(tmp, ".ergo", "workspaces")
	require.NoError(t, os.MkdirAll(wsDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(wsDir, "my-ws.toml"),
		[]byte("[workspace]\nname = \"my-ws\"\n"),
		0o600,
	))

	// Write a .code-workspace file in the temp dir.
	writeCodeWorkspace(t, tmp, "my-ws.code-workspace", "my-ws")

	result, err := Resolve(".", tmp, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Equal(t, "my-ws", result.Name)
}

func TestResolve_DotArg_NotInWorkspace_Error(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No .code-workspace, no known root, git fails.
	_, err := Resolve(".", tmp, fakeRunner{err: errors.New("not a git repo")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not inside an ergo workspace")
}

func TestResolve_NoArg_CWDInWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Place a .code-workspace in the temp dir.
	writeCodeWorkspace(t, tmp, "inferred.code-workspace", "inferred")

	result, err := Resolve("", tmp, fakeRunner{err: errors.New("not a git repo")})
	require.NoError(t, err)
	assert.Equal(t, "inferred", result.Name)
}

func TestResolve_NoArg_OutsideWorkspace_ReturnsCandidates(t *testing.T) {
	tmp := setupWorkspaceNames(t, "alpha", "beta", "gamma")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	// Use a real directory that has no .code-workspace and is outside any workspace root.
	outside := filepath.Join(tmp, "not-a-workspace")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	result, err := Resolve("", outside, noRepo)
	require.NoError(t, err)
	assert.Empty(t, result.Name)
	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, result.Candidates)
}

func TestResolve_GlobPattern_MatchesMultiple(t *testing.T) {
	setupWorkspaceNames(t, "go-tools", "go-experiments", "python-tools")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	result, err := Resolve("go-*", "/tmp", noRepo)
	require.NoError(t, err)
	assert.Empty(t, result.Name)
	assert.ElementsMatch(t, []string{"go-tools", "go-experiments"}, result.Candidates)
}

func TestResolve_GlobPattern_SingleMatch(t *testing.T) {
	setupWorkspaceNames(t, "go-tools", "python-tools")
	noRepo := fakeRunner{err: errors.New("not a git repo")}

	result, err := Resolve("go-*", "/tmp", noRepo)
	require.NoError(t, err)
	assert.Equal(t, "go-tools", result.Name)
}

func TestMatchNames_CaseInsensitive(t *testing.T) {
	names := []string{"ML-Projects", "side-projects"}
	got := matchNames(names, "ml")
	assert.Equal(t, []string{"ML-Projects"}, got)
}

func TestMatchNames_GlobMalformed_FallsBackToSubstring(t *testing.T) {
	names := []string{"ml-projects", "side-projects"}
	// "[unterminated" is a malformed glob; should fall back to substring.
	got := matchNames(names, "[unterminated")
	assert.Empty(t, got) // no name contains "[unterminated"
}

func TestClosestName_ReturnsEmpty_WhenNoPrefix(t *testing.T) {
	names := []string{"alpha", "beta"}
	assert.Empty(t, closestName(names, "zzz"))
}

func TestClosestName_ReturnsClosest(t *testing.T) {
	names := []string{"ml-projects", "side-projects"}
	// "ml-projcts" shares a long prefix with "ml-projects"
	assert.Equal(t, "ml-projects", closestName(names, "ml-projcts"))
}
