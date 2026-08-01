package vscode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/config"
)

func writeWSFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ws.code-workspace")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReadFilter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Filter
		wantErr bool
	}{
		{
			name:    "group filter",
			content: `{"ergo": {"workspace-name": "ws", "filter": {"group": "ml"}}, "folders": []}`,
			want:    &Filter{Group: "ml"},
		},
		{
			name:    "tag filter",
			content: `{"ergo": {"workspace-name": "ws", "filter": {"tags": ["go", "ml"]}}, "folders": []}`,
			want:    &Filter{Tags: []string{"go", "ml"}},
		},
		{
			name:    "no filter key",
			content: `{"ergo": {"workspace-name": "ws"}, "folders": []}`,
			want:    nil,
		},
		{
			name:    "empty filter object treated as no filter",
			content: `{"ergo": {"workspace-name": "ws", "filter": {}}, "folders": []}`,
			want:    nil,
		},
		{
			name:    "unknown fields tolerated",
			content: `{"ergo": {"workspace-name": "ws", "filter": {"group": "ml", "future": true}, "v2": 1}, "folders": [], "extensions": {}}`,
			want:    &Filter{Group: "ml"},
		},
		{
			name:    "no ergo key at all (hand-written workspace file)",
			content: `{"folders": [{"path": "."}]}`,
			want:    nil,
		},
		{
			name:    "malformed json",
			content: `{"ergo": {`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadFilter(writeWSFile(t, tt.content))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadFilter_MissingFile(t *testing.T) {
	_, err := ReadFilter(filepath.Join(t.TempDir(), "nope.code-workspace"))
	require.Error(t, err)
}

// TestReadFilter_RoundTripsGenerate pins the reader to the writer: a filter
// passed through Generate is read back identically.
func TestReadFilter_RoundTripsGenerate(t *testing.T) {
	tests := []struct {
		name   string
		filter *Filter
	}{
		{"group", &Filter{Group: "ml"}},
		{"tags", &Filter{Tags: []string{"go", "ml"}}},
		{"name glob", &Filter{Name: "*-service"}},
		{"nil filter", nil},
	}

	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "ws"},
		Repos:     []config.Repo{{URL: "https://github.com/e/r.git"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Generate(cfg, tt.filter)
			require.NoError(t, err)

			path := writeWSFile(t, string(b))
			got, err := ReadFilter(path)
			require.NoError(t, err)
			assert.Equal(t, tt.filter, got)
		})
	}
}

func TestFilter_Describe(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   string
	}{
		{"group", Filter{Group: "ml"}, `group "ml"`},
		{"single tag", Filter{Tags: []string{"go"}}, `tags "go"`},
		{"multiple tags", Filter{Tags: []string{"go", "ml"}}, `tags "go", "ml"`},
		{"name", Filter{Name: "*-service"}, `name "*-service"`},
		{"combined", Filter{Group: "ml", Tags: []string{"py"}}, `group "ml", tags "py"`},
		{"zero", Filter{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.Describe())
		})
	}
}
