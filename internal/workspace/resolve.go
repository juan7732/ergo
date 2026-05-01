package workspace

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/git"
)

// ResolveResult holds the outcome of workspace resolution.
// Either Name is set (unambiguous) or Candidates is set (TUI selector needed).
type ResolveResult struct {
	// Name is the resolved workspace name. Set when resolution is unambiguous.
	Name string
	// Candidates is a non-nil, non-empty list when resolution is ambiguous —
	// either a partial name matched multiple workspaces (step 2) or no name was
	// provided and the CWD is not inside any workspace (step 6). Phase 4 TUI
	// picks from this list.
	Candidates []string
}

// Resolve implements the six-step Workspace Resolution Order from spec §2.
//
//  1. nameArg exact match → use it.
//  2. nameArg partial/glob match → return candidates (single match resolves directly).
//  3. nameArg provided, no match → error with suggestion.
//  4. nameArg == "." → use current workspace, error if not in one.
//  5. nameArg == "", CWD in workspace → use detected workspace.
//  6. nameArg == "", CWD not in workspace → return all workspaces as candidates.
//
// r is the git runner used internally by Detect; pass git.ExecRunner{} in production.
func Resolve(nameArg, cwd string, r git.Runner) (ResolveResult, error) {
	names, err := config.ListWorkspaceNames()
	if err != nil {
		return ResolveResult{}, fmt.Errorf("listing workspaces: %w", err)
	}

	// Step 4: "." is shorthand for "the workspace I am currently inside".
	if nameArg == "." {
		det, err := Detect(cwd, r)
		if err != nil {
			return ResolveResult{}, err
		}
		if det.WorkspaceName == "" {
			return ResolveResult{}, fmt.Errorf("not inside an ergo workspace; provide a workspace name or run from inside one")
		}
		return ResolveResult{Name: det.WorkspaceName}, nil
	}

	if nameArg != "" {
		// Step 1: exact match.
		for _, n := range names {
			if n == nameArg {
				return ResolveResult{Name: n}, nil
			}
		}

		// Step 2: partial / glob match.
		candidates := matchNames(names, nameArg)
		if len(candidates) > 0 {
			// DECISION: a single unambiguous partial match resolves directly rather
			// than forcing a TUI selection, mirroring how tools like git handle
			// unique branch prefixes. Multiple matches always return candidates.
			if len(candidates) == 1 {
				return ResolveResult{Name: candidates[0]}, nil
			}
			return ResolveResult{Candidates: candidates}, nil
		}

		// Step 3: no match — provide a helpful suggestion when possible.
		if suggestion := closestName(names, nameArg); suggestion != "" {
			return ResolveResult{}, fmt.Errorf("workspace %q not found (did you mean %q?)", nameArg, suggestion)
		}
		return ResolveResult{}, fmt.Errorf("workspace %q not found", nameArg)
	}

	// Steps 5–6: no name argument.
	det, err := Detect(cwd, r)
	if err != nil {
		return ResolveResult{}, err
	}

	// Step 5: detected workspace from CWD.
	if det.WorkspaceName != "" {
		return ResolveResult{Name: det.WorkspaceName}, nil
	}

	// Step 6: outside any workspace — return all workspaces for TUI selector.
	return ResolveResult{Candidates: names}, nil
}

// matchNames returns the subset of names that match pattern.
// If pattern contains glob metacharacters (*, ?, [) it is compiled as a glob
// and matched case-insensitively; otherwise case-insensitive substring matching
// is used.
func matchNames(names []string, pattern string) []string {
	lower := strings.ToLower(pattern)
	if isGlobPattern(lower) {
		g, err := glob.Compile(lower)
		if err != nil {
			// Malformed glob — fall back to substring matching.
			return substringMatch(names, lower)
		}
		var out []string
		for _, n := range names {
			if g.Match(strings.ToLower(n)) {
				out = append(out, n)
			}
		}
		return out
	}
	return substringMatch(names, lower)
}

// isGlobPattern reports whether s contains any glob metacharacter.
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// substringMatch returns names whose lowercase form contains lower as a substring.
func substringMatch(names []string, lower string) []string {
	var out []string
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), lower) {
			out = append(out, n)
		}
	}
	return out
}

// closestName returns the workspace name with the longest common prefix with
// target (case-insensitive). Returns "" when names is empty or no prefix matches
// at least one character.
func closestName(names []string, target string) string {
	tLower := strings.ToLower(target)
	best := ""
	bestScore := 0
	for _, n := range names {
		if score := commonPrefixLen(strings.ToLower(n), tLower); score > bestScore {
			bestScore = score
			best = n
		}
	}
	return best
}

// commonPrefixLen returns the number of bytes in the longest common prefix of a and b.
func commonPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
