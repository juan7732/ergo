package vscode

import (
	"testing"

	"juan7732/ergo/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptr is a helper to get a pointer to a string literal.
func ptr(s string) *string { return &s }

func TestGenerate_Basic(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "test-ws"},
		Repos: []config.Repo{
			{URL: "https://github.com/example/my-repo.git"},
		},
	}

	got, err := Generate(cfg, nil)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "test-ws"
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "my-repo",
      "path": "my-repo"
    }
  ]
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_RootFolderAlwaysFirst(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos: []config.Repo{
			{URL: "https://github.com/example/repo-a.git"},
			{URL: "https://github.com/example/repo-b.git"},
		},
		Folders: []config.Folder{
			{Name: "scratch"},
		},
	}

	got, err := Generate(cfg, nil)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "ws"
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "repo-a",
      "path": "repo-a"
    },
    {
      "name": "repo-b",
      "path": "repo-b"
    },
    {
      "name": "scratch",
      "path": "scratch"
    }
  ]
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_WithActiveFilter(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ml-projects"},
		Repos: []config.Repo{
			{URL: "https://github.com/example/my-repo.git"},
		},
	}
	filter := &Filter{Group: "ml"}

	got, err := Generate(cfg, filter)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "ml-projects",
    "filter": {
      "group": "ml"
    }
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "my-repo",
      "path": "my-repo"
    }
  ]
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_WithTagFilter(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos:     []config.Repo{{URL: "https://github.com/example/repo.git"}},
	}
	filter := &Filter{Tags: []string{"go", "ml"}}

	got, err := Generate(cfg, filter)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "ws",
    "filter": {
      "tags": [
        "go",
        "ml"
      ]
    }
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "repo",
      "path": "repo"
    }
  ]
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_WithWorkspaceAndPerFolderSettings(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{
			Name: "test-ws",
			VSCode: config.VSCodeSection{
				Settings: map[string]any{
					"editor.formatOnSave": true,
				},
			},
		},
		Repos: []config.Repo{
			{
				URL: "https://github.com/example/my-repo.git",
				VSCodeSettings: map[string]any{
					"python.defaultInterpreterPath": ".venv/bin/python",
				},
			},
		},
	}

	got, err := Generate(cfg, nil)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "test-ws"
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "my-repo",
      "path": "my-repo",
      "settings": {
        "python.defaultInterpreterPath": ".venv/bin/python"
      }
    }
  ],
  "settings": {
    "editor.formatOnSave": true
  }
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_ExplicitRepoName(t *testing.T) {
	name := "utils-juan"
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos: []config.Repo{
			{URL: "https://github.com/juan/utils.git", Name: &name},
		},
	}

	got, err := Generate(cfg, nil)
	require.NoError(t, err)

	assert.Contains(t, string(got), `"name": "utils-juan"`)
	assert.Contains(t, string(got), `"path": "utils-juan"`)
}

func TestGenerate_FolderSettings(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Folders: []config.Folder{
			{
				Name: "planning",
				Git:  true,
				VSCodeSettings: map[string]any{
					"editor.wordWrap": "on",
				},
			},
		},
	}

	got, err := Generate(cfg, nil)
	require.NoError(t, err)

	want := `{
  "ergo": {
    "workspace-name": "ws"
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "planning",
      "path": "planning",
      "settings": {
        "editor.wordWrap": "on"
      }
    }
  ]
}
`
	assert.Equal(t, want, string(got))
}

func TestGenerate_ErrorsOnEmptyRepoName(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos:     []config.Repo{{URL: ""}},
	}

	_, err := Generate(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repos[0]")
}

func TestGenerate_TrailingNewline(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
	}
	got, err := Generate(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, byte('\n'), got[len(got)-1], "output should end with a newline")
}

func TestGenerate_NoFilterOmitsFilterKey(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
	}
	got, err := Generate(cfg, nil)
	require.NoError(t, err)
	assert.NotContains(t, string(got), `"filter"`)
}

func TestGenerate_EmptySettingsOmitted(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos: []config.Repo{
			{URL: "https://github.com/example/repo.git", VSCodeSettings: map[string]any{}},
		},
	}
	got, err := Generate(cfg, nil)
	require.NoError(t, err)
	assert.NotContains(t, string(got), `"settings"`)
}
