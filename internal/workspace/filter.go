package workspace

import (
	"strings"

	"github.com/gobwas/glob"

	"juan7732/ergo/internal/config"
)

// FilterOptions specifies which repos to include after applying filters.
// All active filters (Name, Group, Tags) are ANDed together.
// ExcludedGroups are applied only when no explicit filter is active and All is false.
type FilterOptions struct {
	// Name is a glob pattern matched against repo names (case-insensitive).
	// Empty means no name filter.
	Name string

	// Group filters to repos in exactly this group. Empty means no group filter.
	Group string

	// Tags filters to repos that have ANY of these tags. Empty means no tag filter.
	Tags []string

	// IncludeGroup overrides ExcludedGroups for a specific group name.
	// Only meaningful when ExcludedGroups is in effect (no explicit filter active).
	IncludeGroup string

	// All overrides ExcludedGroups when true, including all repos regardless of group.
	All bool

	// ExcludedGroups is the list from [run].excluded_groups in the global config.
	// Applied only when no explicit Name/Group/Tags filter is set and All is false.
	ExcludedGroups []string
}

// ApplyRepoFilter returns the repos from repos that pass opts.
// When opts is the zero value, all repos are returned.
func ApplyRepoFilter(repos []config.Repo, opts FilterOptions) []config.Repo {
	hasExplicit := opts.Name != "" || opts.Group != "" || len(opts.Tags) > 0
	applyExclusion := !hasExplicit && !opts.All

	var nameGlob glob.Glob
	if opts.Name != "" {
		if g, err := glob.Compile(strings.ToLower(opts.Name)); err == nil {
			nameGlob = g
		}
	}

	var out []config.Repo
	for _, repo := range repos {
		// ExcludedGroups check — only when no explicit filter is active.
		if applyExclusion && isInSlice(repo.Group, opts.ExcludedGroups) {
			if opts.IncludeGroup == "" || repo.Group != opts.IncludeGroup {
				continue
			}
		}

		// --name filter: glob match, or case-insensitive substring fallback.
		if opts.Name != "" {
			name := strings.ToLower(repo.EffectiveName())
			if nameGlob != nil {
				if !nameGlob.Match(name) {
					continue
				}
			} else if !strings.Contains(name, strings.ToLower(opts.Name)) {
				continue
			}
		}

		// --group filter: exact match.
		if opts.Group != "" && repo.Group != opts.Group {
			continue
		}

		// --tags filter: any-match.
		if len(opts.Tags) > 0 && !hasAnyTag(repo.Tags, opts.Tags) {
			continue
		}

		out = append(out, repo)
	}
	return out
}

// isInSlice reports whether val appears in slice.
func isInSlice(val string, slice []string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// hasAnyTag reports whether repoTags contains at least one of filterTags.
func hasAnyTag(repoTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, rt := range repoTags {
			if rt == ft {
				return true
			}
		}
	}
	return false
}
