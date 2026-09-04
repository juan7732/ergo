package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/config"
)

// fixtureWorkspaces returns two workspaces that between them exercise every
// match target: effective names (explicit and URL-derived), URLs, folder
// names, and workspace names.
func fixtureWorkspaces() []NamedConfig {
	return []NamedConfig{
		{
			Name: "platform",
			Config: config.WorkspaceConfig{
				Repos: []config.Repo{
					{URL: "https://github.com/juan7732/ergo.git", Group: "core", Tags: []string{"go"}},
					{URL: "https://github.com/other/utils.git", Name: ptr("ergo-utils")},
					{URL: "https://github.com/acme/billing.git"},
				},
				Folders: []config.Folder{{Name: "ergo-notes"}, {Name: "scratch"}},
			},
		},
		{
			Name: "ergo-ecosystem",
			Config: config.WorkspaceConfig{
				Repos: []config.Repo{
					{URL: "https://github.com/ergo-org/corvo.git"},
				},
			},
		},
	}
}

func TestSearch_MatchesEveryTargetAndOrders(t *testing.T) {
	root := t.TempDir()
	hits := Search("ergo", fixtureWorkspaces(), root)

	// Expected order: workspace name, then kind (workspace, repo, folder),
	// then name. "ergo-ecosystem" sorts before "platform".
	require.Len(t, hits, 5)

	assert.Equal(t, Hit{
		Workspace: "ergo-ecosystem", Kind: HitKindWorkspace, Name: "ergo-ecosystem",
		Path: filepath.Join(root, "ergo-ecosystem"),
	}, hits[0])
	assert.Equal(t, Hit{
		Workspace: "ergo-ecosystem", Kind: HitKindRepo, Name: "corvo",
		URL:  "https://github.com/ergo-org/corvo.git",
		Path: filepath.Join(root, "ergo-ecosystem", "corvo"),
	}, hits[1], "corvo matches through its URL, not its name")
	assert.Equal(t, Hit{
		Workspace: "platform", Kind: HitKindRepo, Name: "ergo",
		URL: "https://github.com/juan7732/ergo.git", Group: "core", Tags: []string{"go"},
		Path: filepath.Join(root, "platform", "ergo"),
	}, hits[2])
	assert.Equal(t, Hit{
		Workspace: "platform", Kind: HitKindRepo, Name: "ergo-utils",
		URL:  "https://github.com/other/utils.git",
		Path: filepath.Join(root, "platform", "ergo-utils"),
	}, hits[3], "explicit name wins over the URL-derived one")
	assert.Equal(t, Hit{
		Workspace: "platform", Kind: HitKindFolder, Name: "ergo-notes",
		Path: filepath.Join(root, "platform", "ergo-notes"),
	}, hits[4])
}

func TestSearch_CaseInsensitive(t *testing.T) {
	root := t.TempDir()
	lower := Search("ergo", fixtureWorkspaces(), root)
	upper := Search("ERGO", fixtureWorkspaces(), root)
	mixed := Search("ErGo", fixtureWorkspaces(), root)
	assert.Equal(t, lower, upper)
	assert.Equal(t, lower, mixed)
}

func TestSearch_URLOnlyMatch(t *testing.T) {
	hits := Search("acme", fixtureWorkspaces(), t.TempDir())
	require.Len(t, hits, 1)
	assert.Equal(t, HitKindRepo, hits[0].Kind)
	assert.Equal(t, "billing", hits[0].Name)
}

func TestSearch_NoMatchIsEmpty(t *testing.T) {
	hits := Search("nothing-here", fixtureWorkspaces(), t.TempDir())
	assert.Empty(t, hits)
}

func TestSearch_NoWorkspaces(t *testing.T) {
	assert.Empty(t, Search("ergo", nil, t.TempDir()))
}

func TestSearch_OnDiskState(t *testing.T) {
	root := t.TempDir()
	ws := fixtureWorkspaces()

	// Nothing materialized: every hit reports absent.
	for _, h := range Search("ergo", ws, root) {
		assert.False(t, h.Exists, "%s %s should not exist yet", h.Kind, h.Name)
	}

	// Materialize the platform workspace: a cloned repo (with .git), a repo
	// directory without .git (not cloned), and the folder.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "platform", "ergo", ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "platform", "ergo-utils"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "platform", "ergo-notes"), 0o755))

	byKey := map[string]Hit{}
	for _, h := range Search("ergo", ws, root) {
		byKey[h.Workspace+"/"+string(h.Kind)+"/"+h.Name] = h
	}

	assert.True(t, byKey["platform/repo/ergo"].Exists, "repo with .git is cloned")
	assert.False(t, byKey["platform/repo/ergo-utils"].Exists, "a bare directory without .git is not cloned")
	assert.True(t, byKey["platform/folder/ergo-notes"].Exists, "folder directory exists")
	assert.False(t, byKey["ergo-ecosystem/workspace/ergo-ecosystem"].Exists, "workspace directory absent")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "ergo-ecosystem"), 0o755))
	for _, h := range Search("ergo-ecosystem", ws, root) {
		if h.Kind == HitKindWorkspace {
			assert.True(t, h.Exists, "workspace directory now exists")
		}
	}
}

func TestSearch_WorktreeGitFileCountsAsCloned(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "platform", "ergo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: elsewhere\n"), 0o644))

	hits := Search("juan7732/ergo", fixtureWorkspaces(), root)
	require.Len(t, hits, 1)
	assert.True(t, hits[0].Exists)
}

// An empty query is the full index: every workspace, repo, and folder, in
// the same order a non-empty query would list them. `ergo search --json`
// with no query relies on this instead of a separate index path.
func TestSearch_EmptyQueryReturnsEverything(t *testing.T) {
	hits := Search("", fixtureWorkspaces(), t.TempDir())

	var got []string
	for _, h := range hits {
		got = append(got, h.Workspace+"/"+string(h.Kind)+"/"+h.Name)
	}
	assert.Equal(t, []string{
		"ergo-ecosystem/workspace/ergo-ecosystem",
		"ergo-ecosystem/repo/corvo",
		"platform/workspace/platform",
		"platform/repo/billing",
		"platform/repo/ergo",
		"platform/repo/ergo-utils",
		"platform/folder/ergo-notes",
		"platform/folder/scratch",
	}, got)
}
