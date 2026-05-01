package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"juan7732/ergo/internal/config"
)

func ptr(s string) *string { return &s }

func TestApplyRepoFilter_NoFilter(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/repo-a.git", Group: "ml", Tags: []string{"go"}},
		{URL: "https://github.com/example/repo-b.git", Group: "tools"},
	}

	got := ApplyRepoFilter(repos, FilterOptions{})
	assert.Equal(t, repos, got, "zero-value filter should return all repos")
}

func TestApplyRepoFilter_GroupFilter(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "ml"},
		{URL: "https://github.com/example/b.git", Group: "tools"},
		{URL: "https://github.com/example/c.git", Group: "ml"},
	}

	got := ApplyRepoFilter(repos, FilterOptions{Group: "ml"})
	assert.Len(t, got, 2)
	assert.Equal(t, "a", got[0].EffectiveName())
	assert.Equal(t, "c", got[1].EffectiveName())
}

func TestApplyRepoFilter_TagsFilter_AnyMatch(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Tags: []string{"go", "ml"}},
		{URL: "https://github.com/example/b.git", Tags: []string{"python"}},
		{URL: "https://github.com/example/c.git", Tags: []string{"go"}},
	}

	// "go" should match a and c
	got := ApplyRepoFilter(repos, FilterOptions{Tags: []string{"go"}})
	assert.Len(t, got, 2)
	assert.Equal(t, "a", got[0].EffectiveName())
	assert.Equal(t, "c", got[1].EffectiveName())

	// "python" or "ml" should match a and b
	got = ApplyRepoFilter(repos, FilterOptions{Tags: []string{"python", "ml"}})
	assert.Len(t, got, 2)
}

func TestApplyRepoFilter_NameFilter_ExactGlob(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/kb-core.git"},
		{URL: "https://github.com/example/kb-python.git"},
		{URL: "https://github.com/example/ergo.git"},
	}

	// glob: kb-* matches kb-core and kb-python
	got := ApplyRepoFilter(repos, FilterOptions{Name: "kb-*"})
	assert.Len(t, got, 2)

	// substring fallback (no glob chars): "ergo"
	got = ApplyRepoFilter(repos, FilterOptions{Name: "ergo"})
	assert.Len(t, got, 1)
	assert.Equal(t, "ergo", got[0].EffectiveName())
}

func TestApplyRepoFilter_NameFilter_CaseInsensitive(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/MyRepo.git"},
	}

	got := ApplyRepoFilter(repos, FilterOptions{Name: "myrepo"})
	assert.Len(t, got, 1)
}

func TestApplyRepoFilter_ExcludedGroups_AppliedWhenNoExplicitFilter(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "ml"},
		{URL: "https://github.com/example/b.git", Group: "documentation"},
		{URL: "https://github.com/example/c.git", Group: "tools"},
	}

	got := ApplyRepoFilter(repos, FilterOptions{
		ExcludedGroups: []string{"documentation", "tools"},
	})
	assert.Len(t, got, 1)
	assert.Equal(t, "a", got[0].EffectiveName())
}

func TestApplyRepoFilter_ExcludedGroups_OverriddenByExplicitFilter(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "documentation"},
		{URL: "https://github.com/example/b.git", Group: "ml"},
	}

	// Explicit group filter should ignore ExcludedGroups.
	got := ApplyRepoFilter(repos, FilterOptions{
		Group:          "documentation",
		ExcludedGroups: []string{"documentation"},
	})
	assert.Len(t, got, 1)
	assert.Equal(t, "a", got[0].EffectiveName())
}

func TestApplyRepoFilter_All_IgnoresExcludedGroups(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "documentation"},
		{URL: "https://github.com/example/b.git", Group: "tools"},
	}

	got := ApplyRepoFilter(repos, FilterOptions{
		All:            true,
		ExcludedGroups: []string{"documentation", "tools"},
	})
	assert.Len(t, got, 2)
}

func TestApplyRepoFilter_IncludeGroup_OverridesExclusion(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "documentation"},
		{URL: "https://github.com/example/b.git", Group: "tools"},
		{URL: "https://github.com/example/c.git", Group: "ml"},
	}

	// No explicit filter, but documentation is excluded and tools is included via IncludeGroup.
	got := ApplyRepoFilter(repos, FilterOptions{
		ExcludedGroups: []string{"documentation", "tools"},
		IncludeGroup:   "tools",
	})
	assert.Len(t, got, 2)
	assert.Equal(t, "b", got[0].EffectiveName())
	assert.Equal(t, "c", got[1].EffectiveName())
}

func TestApplyRepoFilter_AndSemantics_GroupAndTags(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/a.git", Group: "ml", Tags: []string{"python"}},
		{URL: "https://github.com/example/b.git", Group: "ml", Tags: []string{"go"}},
		{URL: "https://github.com/example/c.git", Group: "tools", Tags: []string{"python"}},
	}

	// group=ml AND tags=python → only a
	got := ApplyRepoFilter(repos, FilterOptions{Group: "ml", Tags: []string{"python"}})
	assert.Len(t, got, 1)
	assert.Equal(t, "a", got[0].EffectiveName())
}

func TestApplyRepoFilter_ExplicitName_InhibitsExcludedGroups(t *testing.T) {
	repos := []config.Repo{
		{URL: "https://github.com/example/kb-core.git", Group: "documentation"},
	}

	// Explicit --name should override excluded_groups.
	got := ApplyRepoFilter(repos, FilterOptions{
		Name:           "kb-core",
		ExcludedGroups: []string{"documentation"},
	})
	assert.Len(t, got, 1)
}
