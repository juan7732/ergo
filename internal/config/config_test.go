package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeriveRepoName covers URL → directory name derivation.
func TestDeriveRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "https with .git suffix",
			url:  "https://github.com/juan/handwriting-recognition.git",
			want: "handwriting-recognition",
		},
		{
			name: "https without .git suffix",
			url:  "https://github.com/juan/myrepo",
			want: "myrepo",
		},
		{
			name: "scp-style git URL",
			url:  "git@github.com:juan/ergo.git",
			want: "ergo",
		},
		{
			name: "scp-style without .git",
			url:  "git@github.com:juan/ergo",
			want: "ergo",
		},
		{
			name: "empty url",
			url:  "",
			want: "",
		},
		{
			name: "trailing slash after .git stripped",
			url:  "https://github.com/juan/repo.git/",
			want: "repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DeriveRepoName(tc.url))
		})
	}
}

// TestRepo_EffectiveName checks that explicit name wins over derivation.
func TestRepo_EffectiveName(t *testing.T) {
	t.Run("explicit name wins", func(t *testing.T) {
		name := "utils-juan"
		r := Repo{URL: "https://github.com/juan/utils.git", Name: &name}
		assert.Equal(t, "utils-juan", r.EffectiveName())
	})

	t.Run("nil name triggers derivation", func(t *testing.T) {
		r := Repo{URL: "https://github.com/juan/handwriting-recognition.git"}
		assert.Equal(t, "handwriting-recognition", r.EffectiveName())
	})

	t.Run("empty url with nil name", func(t *testing.T) {
		r := Repo{}
		assert.Equal(t, "", r.EffectiveName())
	})
}

// TestLoadGlobal_CreatesDefaultWhenMissing verifies that LoadGlobal creates
// ~/.ergo/config.toml with defaults when it doesn't exist yet.
func TestLoadGlobal_CreatesDefaultWhenMissing(t *testing.T) {
	// Point HOME to a temp dir so we don't touch the real ~/.ergo.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := LoadGlobal()
	require.NoError(t, err)

	assert.Equal(t, "~/ergo-workspaces", cfg.Defaults.WorkspaceRoot)
	assert.Equal(t, "main", cfg.Defaults.DefaultBranch)
	assert.True(t, cfg.Parallel.Enabled)
	assert.Equal(t, 4, cfg.Parallel.BatchSize)
	assert.True(t, cfg.Sync.AutoPull)
	assert.Equal(t, GitProtocolHTTPS, cfg.Git.Protocol)

	// Config file must have been created and self-document the [git] section.
	body, readErr := os.ReadFile(filepath.Join(tmp, ".ergo", "config.toml"))
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "[git]")
}

// TestLoadGlobal_MissingGitSectionMeansHTTPS is a regression test for the
// zero-value gotcha: existing config files without a [git] section must decode
// to "no SSH rewriting", not an error or surprise rewrite.
func TestLoadGlobal_MissingGitSectionMeansHTTPS(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".ergo")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	body := "[defaults]\nworkspace_root = \"~/ergo-workspaces\"\ndefault_branch = \"main\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o600))

	cfg, err := LoadGlobal()
	require.NoError(t, err)
	assert.Empty(t, cfg.Git.Protocol)
	assert.False(t, cfg.Git.UseSSH())
}

// TestLoadGlobal_RejectsInvalidGitProtocol verifies unknown protocol values
// fail fast with a clear error.
func TestLoadGlobal_RejectsInvalidGitProtocol(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".ergo")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	body := "[git]\nprotocol = \"carrier-pigeon\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o600))

	_, err := LoadGlobal()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carrier-pigeon")
	assert.Contains(t, err.Error(), "[git]")
}

// TestGitConfig_UseSSH covers the protocol → rewrite-enabled mapping.
func TestGitConfig_UseSSH(t *testing.T) {
	tests := []struct {
		protocol string
		want     bool
	}{
		{"", false},
		{"https", false},
		{"ssh", true},
		{"SSH ", true},
		{" Ssh", true},
	}
	for _, tc := range tests {
		t.Run("protocol="+tc.protocol, func(t *testing.T) {
			assert.Equal(t, tc.want, GitConfig{Protocol: tc.protocol}.UseSSH())
		})
	}
}

// TestLoadGlobal_RoundTrip verifies that a written config can be read back.
func TestLoadGlobal_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// First load creates the file with defaults.
	first, err := LoadGlobal()
	require.NoError(t, err)

	// Second load reads the file.
	second, err := LoadGlobal()
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

// TestValidate_Valid verifies that a well-formed config produces no errors.
func TestValidate_Valid(t *testing.T) {
	cfg := WorkspaceConfig{
		Workspace: WorkspaceMeta{Name: "ml-projects"},
		Repos: []Repo{
			{URL: "https://github.com/juan/ergo.git"},
			{URL: "https://github.com/juan/kb-core.git"},
		},
		Folders: []Folder{
			{Name: "scratch"},
		},
	}
	assert.NoError(t, Validate(cfg))
}

// TestValidate_MissingURL checks that repos without a URL are flagged.
func TestValidate_MissingURL(t *testing.T) {
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: ""},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	require.Len(t, ve, 1)
	assert.Contains(t, ve[0].Message, "url is required")
}

// TestValidate_RepoNameCollision checks that two repos deriving the same name are flagged.
func TestValidate_RepoNameCollision(t *testing.T) {
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: "https://github.com/juan/utils.git"},
			{URL: "https://github.com/other/utils.git"},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	assert.Len(t, ve, 1)
	assert.Contains(t, ve[0].Message, "collides")
}

// TestValidate_ExplicitNamesResolveCollision verifies that providing distinct
// explicit names makes the collision go away.
func TestValidate_ExplicitNamesResolveCollision(t *testing.T) {
	nameA := "utils-juan"
	nameB := "utils-other"
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: "https://github.com/juan/utils.git", Name: &nameA},
			{URL: "https://github.com/other/utils.git", Name: &nameB},
		},
	}
	assert.NoError(t, Validate(cfg))
}

// TestValidate_RepoFolderCollision checks that a folder with the same name as a
// repo is flagged.
func TestValidate_RepoFolderCollision(t *testing.T) {
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: "https://github.com/juan/scratch.git"},
		},
		Folders: []Folder{
			{Name: "scratch"},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	require.Len(t, ve, 1)
	assert.Contains(t, ve[0].Message, "collides with a repo")
}

// TestValidate_DuplicateFolder checks that two folders with the same name are flagged.
func TestValidate_DuplicateFolder(t *testing.T) {
	cfg := WorkspaceConfig{
		Folders: []Folder{
			{Name: "notes"},
			{Name: "notes"},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	require.Len(t, ve, 1)
	assert.Contains(t, ve[0].Message, "duplicated")
}

// TestValidate_EmptyTag checks that an empty string in the tags slice is flagged.
func TestValidate_EmptyTag(t *testing.T) {
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: "https://github.com/juan/ergo.git", Tags: []string{"go", ""}},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	require.Len(t, ve, 1)
	assert.Contains(t, ve[0].Message, "tag must not be empty")
}

// TestValidate_MultipleErrors verifies that all errors are collected, not just the first.
func TestValidate_MultipleErrors(t *testing.T) {
	cfg := WorkspaceConfig{
		Repos: []Repo{
			{URL: ""},
			{URL: ""},
		},
	}
	err := Validate(cfg)
	require.Error(t, err)
	ve, ok := err.(ValidationErrors)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(ve), 2)
}
