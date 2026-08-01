package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/vscode"
	"github.com/juan7732/ergo/internal/workspace"
)

// The golden strings below are the shipped wire format (ergo-vscode-spec.md
// §3.1). Changing any of them is a breaking change to the JSON contract —
// additions only.

func TestMarshal_Status_Golden(t *testing.T) {
	tests := []struct {
		name string
		doc  Status
		want string
	}{
		{
			name: "workspace with one repo",
			doc: NewStatus("ml-projects", []workspace.RepoStatusEntry{
				{
					Name:   "handwriting-recognition",
					Branch: "dev",
					Dirty:  true,
					Behind: 3,
					Group:  "ml",
					Tags:   []string{"ml", "python"},
				},
			}),
			want: `{
  "workspace": "ml-projects",
  "repos": [
    {
      "name": "handwriting-recognition",
      "branch": "dev",
      "dirty": true,
      "behind": 3,
      "uncloned": false,
      "group": "ml",
      "tags": [
        "ml",
        "python"
      ]
    }
  ]
}
`,
		},
		{
			name: "uncloned repo has empty branch and zero counts",
			doc: NewStatus("ws", []workspace.RepoStatusEntry{
				{Name: "absent", Uncloned: true},
			}),
			want: `{
  "workspace": "ws",
  "repos": [
    {
      "name": "absent",
      "branch": "",
      "dirty": false,
      "behind": 0,
      "uncloned": true,
      "group": "",
      "tags": []
    }
  ]
}
`,
		},
		{
			name: "standalone repo mode: empty workspace, nil tags become []",
			doc: NewStatus("", []workspace.RepoStatusEntry{
				{Name: "solo", Branch: "main"},
			}),
			want: `{
  "workspace": "",
  "repos": [
    {
      "name": "solo",
      "branch": "main",
      "dirty": false,
      "behind": 0,
      "uncloned": false,
      "group": "",
      "tags": []
    }
  ]
}
`,
		},
		{
			name: "no repos matched filter: empty array, not null",
			doc:  NewStatus("ws", nil),
			want: `{
  "workspace": "ws",
  "repos": []
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.doc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestMarshal_List_Golden(t *testing.T) {
	tests := []struct {
		name string
		doc  List
		want string
	}{
		{
			name: "one workspace",
			doc: NewList([]ListWorkspace{
				{Name: "ml-projects", Repos: 4, Synced: true},
			}),
			want: `{
  "workspaces": [
    {
      "name": "ml-projects",
      "repos": 4,
      "synced": true
    }
  ]
}
`,
		},
		{
			name: "empty state is an empty array, not the human hint text",
			doc:  NewList(nil),
			want: `{
  "workspaces": []
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.doc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestMarshal_Validate_Golden(t *testing.T) {
	tests := []struct {
		name string
		doc  Validate
		want string
	}{
		{
			name: "invalid with one field error",
			doc: NewValidate("ml-projects", config.ValidationErrors{
				{Field: "repos[2]", Message: `derived name "utils" collides with repos[0]`},
			}),
			want: `{
  "workspace": "ml-projects",
  "valid": false,
  "errors": [
    {
      "field": "repos[2]",
      "message": "derived name \"utils\" collides with repos[0]"
    }
  ]
}
`,
		},
		{
			name: "valid: errors present and empty",
			doc:  NewValidate("ws", nil),
			want: `{
  "workspace": "ws",
  "valid": true,
  "errors": []
}
`,
		},
		{
			name: "TOML parse failure: single error with empty field",
			doc:  NewValidate("broken", assertErr("parsing workspace config: toml: line 3: expected key")),
			want: `{
  "workspace": "broken",
  "valid": false,
  "errors": [
    {
      "field": "",
      "message": "parsing workspace config: toml: line 3: expected key"
    }
  ]
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.doc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestMarshal_ValidateAll_Golden(t *testing.T) {
	doc := NewValidateAll([]Validate{
		NewValidate("a", nil),
		NewValidate("b", config.ValidationErrors{{Field: "repos[0]", Message: "url is required"}}),
	})
	got, err := Marshal(doc)
	require.NoError(t, err)

	want := `{
  "workspaces": [
    {
      "workspace": "a",
      "valid": true,
      "errors": []
    },
    {
      "workspace": "b",
      "valid": false,
      "errors": [
        {
          "field": "repos[0]",
          "message": "url is required"
        }
      ]
    }
  ]
}
`
	assert.Equal(t, want, string(got))

	empty, err := Marshal(NewValidateAll(nil))
	require.NoError(t, err)
	assert.Equal(t, "{\n  \"workspaces\": []\n}\n", string(empty))
}

func TestMarshal_Config_Golden(t *testing.T) {
	tests := []struct {
		name string
		doc  Config
		want string
	}{
		{
			name: "repos and folders with derived and explicit names",
			doc: NewConfig("ml-projects", config.WorkspaceConfig{
				Repos: []config.Repo{
					{
						URL:   "https://github.com/juan/handwriting-recognition.git",
						Tags:  []string{"ml", "python"},
						Group: "ml",
					},
					{
						URL:  "https://github.com/other-org/utils.git",
						Name: ptrStr("utils-other"),
					},
				},
				Folders: []config.Folder{
					{Name: "scratch"},
					{Name: "planning", Git: true},
				},
			}),
			want: `{
  "workspace": "ml-projects",
  "repos": [
    {
      "name": "handwriting-recognition",
      "url": "https://github.com/juan/handwriting-recognition.git",
      "tags": [
        "ml",
        "python"
      ],
      "group": "ml"
    },
    {
      "name": "utils-other",
      "url": "https://github.com/other-org/utils.git",
      "tags": [],
      "group": ""
    }
  ],
  "folders": [
    {
      "name": "scratch",
      "git": false
    },
    {
      "name": "planning",
      "git": true
    }
  ]
}
`,
		},
		{
			name: "empty workspace: arrays present and empty",
			doc:  NewConfig("bare", config.WorkspaceConfig{}),
			want: `{
  "workspace": "bare",
  "repos": [],
  "folders": []
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.doc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func ptrStr(s string) *string { return &s }

func TestMarshal_Show_Golden(t *testing.T) {
	tests := []struct {
		name string
		doc  Show
		want string
	}{
		{
			name: "group filter",
			doc:  NewShow("ml-projects", &vscode.Filter{Group: "ml"}),
			want: `{
  "workspace": "ml-projects",
  "filter": {
    "group": "ml"
  }
}
`,
		},
		{
			name: "tag filter",
			doc:  NewShow("ws", &vscode.Filter{Tags: []string{"go", "ml"}}),
			want: `{
  "workspace": "ws",
  "filter": {
    "tags": [
      "go",
      "ml"
    ]
  }
}
`,
		},
		{
			name: "no active filter is null",
			doc:  NewShow("ws", nil),
			want: `{
  "workspace": "ws",
  "filter": null
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.doc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// assertErr wraps a plain string as an error for parse-failure test cases.
func assertErr(msg string) error { return errString(msg) }

type errString string

func (e errString) Error() string { return string(e) }
