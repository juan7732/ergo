package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juan7732/ergo/internal/config"
)

// HitKind discriminates what a search hit refers to.
type HitKind string

const (
	HitKindRepo      HitKind = "repo"
	HitKindFolder    HitKind = "folder"
	HitKindWorkspace HitKind = "workspace"
)

// NamedConfig pairs a workspace name with its parsed TOML. The name comes
// from the file name under ~/.ergo/workspaces/, which is what every other
// command treats as the workspace's identity.
type NamedConfig struct {
	Name   string
	Config config.WorkspaceConfig
}

// Hit is one search match: a repo, folder, or workspace whose name (or URL,
// for repos) contains the query.
type Hit struct {
	// Workspace is the name of the workspace the hit belongs to. For a
	// workspace hit it equals Name.
	Workspace string
	Kind      HitKind
	// Name is the repo's effective name, the folder name, or the workspace
	// name, depending on Kind.
	Name string
	// URL, Group, and Tags are populated for repo hits only. Tags is nil
	// when the TOML defines none; the output layer normalizes that.
	URL   string
	Group string
	Tags  []string
	// Path is the absolute on-disk location the hit would occupy, whether
	// or not it exists yet: <wsRoot>/<ws>/<name> for repos and folders,
	// <wsRoot>/<ws> for workspaces.
	Path string
	// Exists reports the on-disk state for the kind: cloned (a .git entry
	// exists under Path) for repos, directory exists for folders and
	// workspaces. Mirrors the checks sync and list already make.
	Exists bool
}

// Search returns every repo, folder, and workspace across workspaces whose
// name (or URL, for repos) contains query, ignoring case. Results are
// ordered by workspace name, then kind (workspace, repo, folder), then name.
// An empty result is a nil slice; callers that need [] normalize it.
//
// The only filesystem access is os.Stat on the computed paths under wsRoot,
// so tests can drive it with t.TempDir().
func Search(query string, workspaces []NamedConfig, wsRoot string) []Hit {
	// DECISION: plain case-insensitive substring match rather than the glob
	// syntax --name uses. Search is a recall question ("is ergo anywhere?"),
	// where forgetting to wrap the query in * would be a footgun and glob
	// precision buys nothing. Glob support is additive if substring hurts.
	needle := strings.ToLower(query)
	matches := func(s string) bool {
		return strings.Contains(strings.ToLower(s), needle)
	}

	var hits []Hit
	for _, ws := range workspaces {
		wsDir := filepath.Join(wsRoot, ws.Name)

		if matches(ws.Name) {
			hits = append(hits, Hit{
				Workspace: ws.Name,
				Kind:      HitKindWorkspace,
				Name:      ws.Name,
				Path:      wsDir,
				Exists:    isDir(wsDir),
			})
		}

		for _, repo := range ws.Config.Repos {
			name := repo.EffectiveName()
			if !matches(name) && !matches(repo.URL) {
				continue
			}
			dir := filepath.Join(wsDir, name)
			hits = append(hits, Hit{
				Workspace: ws.Name,
				Kind:      HitKindRepo,
				Name:      name,
				URL:       repo.URL,
				Group:     repo.Group,
				Tags:      repo.Tags,
				Path:      dir,
				Exists:    isCloned(dir),
			})
		}

		for _, folder := range ws.Config.Folders {
			if !matches(folder.Name) {
				continue
			}
			dir := filepath.Join(wsDir, folder.Name)
			hits = append(hits, Hit{
				Workspace: ws.Name,
				Kind:      HitKindFolder,
				Name:      folder.Name,
				Path:      dir,
				Exists:    isDir(dir),
			})
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Workspace != b.Workspace {
			return a.Workspace < b.Workspace
		}
		if a.Kind != b.Kind {
			return kindRank(a.Kind) < kindRank(b.Kind)
		}
		return a.Name < b.Name
	})
	return hits
}

// kindRank orders kinds within one workspace: the workspace itself first,
// then its repos, then its folders.
//
// DECISION: the plan fixes the sort keys (workspace, kind, name) but not the
// order of kinds. Container before contents reads naturally in the table;
// alphabetical kind order would interleave folders before repos.
func kindRank(k HitKind) int {
	switch k {
	case HitKindWorkspace:
		return 0
	case HitKindRepo:
		return 1
	default:
		return 2
	}
}

// isDir reports whether path exists and is a directory. Same check as
// `ergo list` uses for its synced column.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isCloned reports whether dir holds a git checkout: a .git entry exists
// (directory, or file for worktrees). Same check syncFolder makes before
// deciding whether to git init.
func isCloned(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
