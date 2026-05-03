package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/git"
)

// Detection describes the ergo context inferred from the working directory.
type Detection struct {
	// WorkspaceName is the detected ergo workspace name. Empty when not inside a workspace.
	WorkspaceName string
	// IsStandaloneRepo is true when CWD is inside a git repo but not an ergo workspace.
	IsStandaloneRepo bool
	// StandaloneRepoRoot is the absolute git root path when IsStandaloneRepo is true.
	StandaloneRepoRoot string
}

// Detect determines the ergo context for cwd by trying detection strategies in order:
//  1. Walk up the directory tree looking for a *.code-workspace file with an "ergo" key.
//  2. Match CWD against known workspace roots from the global config.
//  3. Standalone git repo detection via git rev-parse --show-toplevel.
//
// r is the git runner used for standalone-repo detection; pass git.ExecRunner{} in
// production code.
func Detect(cwd string, r git.Runner) (Detection, error) {
	// Strategy 1: walk up looking for *.code-workspace with "ergo" key.
	name, err := findCodeWorkspace(cwd)
	if err != nil {
		return Detection{}, err
	}
	if name != "" {
		return Detection{WorkspaceName: name}, nil
	}

	// Strategy 2: CWD is under a known workspace root from config.
	name, err = matchKnownRoot(cwd)
	if err != nil {
		return Detection{}, err
	}
	if name != "" {
		return Detection{WorkspaceName: name}, nil
	}

	// Strategy 3: standalone git repo — not an ergo workspace.
	root, err := git.RepoRoot(r, cwd)
	if err == nil && root != "" {
		return Detection{IsStandaloneRepo: true, StandaloneRepoRoot: root}, nil
	}

	// Neither a workspace nor a standalone repo.
	return Detection{}, nil
}

// ergoCodeWorkspace is the minimal structure needed to check for the "ergo" key
// in a .code-workspace file and extract the workspace name.
type ergoCodeWorkspace struct {
	Ergo *ergoMeta `json:"ergo"`
}

type ergoMeta struct {
	WorkspaceName string `json:"workspace-name"`
}

// findCodeWorkspace walks up from dir looking for a *.code-workspace file that
// contains an "ergo" key with a workspace-name. Returns the workspace name, or
// an empty string if no such file is found in any ancestor directory.
func findCodeWorkspace(dir string) (string, error) {
	current := filepath.Clean(dir)
	for {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", fmt.Errorf("reading directory %s: %w", current, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".code-workspace") {
				continue
			}
			name, ok, err := readErgoWorkspaceName(filepath.Join(current, e.Name()))
			if err != nil {
				// Unreadable or malformed — skip silently; not our file.
				continue
			}
			if ok && name != "" {
				return name, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", nil
}

// readErgoWorkspaceName parses a .code-workspace file and returns the
// workspace-name embedded in its "ergo" key.
// Returns ("", false, nil) when the file is valid JSON but has no "ergo" key.
func readErgoWorkspaceName(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", path, err)
	}
	var ws ergoCodeWorkspace
	if err := json.Unmarshal(data, &ws); err != nil {
		// Malformed JSON — not a file we generated.
		return "", false, nil
	}
	if ws.Ergo == nil {
		return "", false, nil
	}
	return ws.Ergo.WorkspaceName, true, nil
}

// matchKnownRoot checks whether cwd sits inside a directory that corresponds to
// a known workspace. It derives the expected workspace directory as
// <workspace_root>/<workspace-name> and checks whether cwd starts with that path.
func matchKnownRoot(cwd string) (string, error) {
	gcfg, err := config.LoadGlobal()
	if err != nil {
		return "", fmt.Errorf("loading global config: %w", err)
	}

	wsRoot, err := config.ExpandTilde(gcfg.Defaults.WorkspaceRoot)
	if err != nil {
		return "", fmt.Errorf("expanding workspace root: %w", err)
	}
	wsRoot = filepath.Clean(wsRoot)

	// cwd must be a subdirectory of wsRoot.
	sep := string(filepath.Separator)
	prefix := wsRoot + sep
	if !strings.HasPrefix(cwd, prefix) {
		return "", nil
	}

	// Extract the immediate subdirectory name — that is the workspace name.
	rel := strings.TrimPrefix(cwd, prefix)
	parts := strings.SplitN(rel, sep, 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", nil
	}
	candidate := parts[0]

	// Cross-reference against actual workspace TOML files.
	names, err := config.ListWorkspaceNames()
	if err != nil {
		return "", fmt.Errorf("listing workspace names: %w", err)
	}
	for _, name := range names {
		if name == candidate {
			return name, nil
		}
	}
	return "", nil
}
