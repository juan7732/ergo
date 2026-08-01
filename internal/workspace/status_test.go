package workspace

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/internal/config"
)

// fakeStatusRunner answers the git queries gatherRepoStatus issues with
// canned values: branch "main", clean tree, behind 0.
type fakeStatusRunner struct{}

func (fakeStatusRunner) Run(dir, name string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "rev-list" {
		return "0", nil
	}
	if len(args) > 1 && args[0] == "rev-parse" {
		return "main", nil
	}
	return "", nil
}

func TestGatherStatus_PlumbsGroupAndTags(t *testing.T) {
	tests := []struct {
		name      string
		repo      config.Repo
		wantGroup string
		wantTags  []string
	}{
		{
			name:      "group and tags carried through",
			repo:      config.Repo{URL: "https://github.com/e/a.git", Group: "ml", Tags: []string{"ml", "python"}},
			wantGroup: "ml",
			wantTags:  []string{"ml", "python"},
		},
		{
			name:      "no group no tags",
			repo:      config.Repo{URL: "https://github.com/e/b.git"},
			wantGroup: "",
			wantTags:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.WorkspaceConfig{Repos: []config.Repo{tt.repo}}
			// wsDir points at a nonexistent path, so every repo is uncloned —
			// group/tags plumbing is independent of on-disk state.
			entries, err := GatherStatus(cfg, t.TempDir()+"/missing", fakeStatusRunner{}, false, 1)
			require.NoError(t, err)
			require.Len(t, entries, 1)

			assert.True(t, entries[0].Uncloned)
			assert.Equal(t, tt.wantGroup, entries[0].Group)
			assert.Equal(t, tt.wantTags, entries[0].Tags)
		})
	}
}

func TestGatherStatus_ParallelKeepsOrderAndTags(t *testing.T) {
	repos := make([]config.Repo, 6)
	for i := range repos {
		repos[i] = config.Repo{
			URL:  fmt.Sprintf("https://github.com/e/repo-%d.git", i),
			Tags: []string{fmt.Sprintf("t%d", i)},
		}
	}
	cfg := config.WorkspaceConfig{Repos: repos}

	entries, err := GatherStatus(cfg, t.TempDir()+"/missing", fakeStatusRunner{}, true, 4)
	require.NoError(t, err)
	require.Len(t, entries, len(repos))

	for i, e := range entries {
		assert.Equal(t, fmt.Sprintf("repo-%d", i), e.Name, "entries must stay in TOML order")
		assert.Equal(t, []string{fmt.Sprintf("t%d", i)}, e.Tags)
	}
}

func TestGatherSingleRepoStatus_NoTags(t *testing.T) {
	entry := GatherSingleRepoStatus(t.TempDir()+"/missing", "solo", "", fakeStatusRunner{})
	assert.True(t, entry.Uncloned)
	assert.Nil(t, entry.Tags)
}
