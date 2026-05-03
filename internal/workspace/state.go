package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/juan7732/ergo/internal/config"
)

// RepoStateEntry holds cached state for a single repo.
// CommitHash is not populated in v1 (would require an extra git rev-parse
// call per repo for a value never read in v1).
// TODO: populate CommitHash once state is consumed by status or other callers.
type RepoStateEntry struct {
	LastPulled time.Time `json:"last_pulled"`
	CommitHash string    `json:"commit_hash,omitempty"`
}

// WorkspaceState is the cached state for a workspace, persisted to
// ~/.ergo/state/<workspace>.json. It is a performance optimization only —
// ergo must function correctly when the file is missing or corrupt.
type WorkspaceState struct {
	Workspace string                    `json:"workspace"`
	LastSync  time.Time                 `json:"last_sync"`
	Repos     map[string]RepoStateEntry `json:"repos"`
}

// LoadState reads the cached state for wsName from ~/.ergo/state/<wsName>.json.
// If the file is missing or contains invalid JSON, a zero-value WorkspaceState
// is returned without error — callers must not depend on this data being present.
func LoadState(wsName string) (WorkspaceState, error) {
	path, err := statePath(wsName)
	if err != nil {
		return WorkspaceState{}, nil // graceful degradation
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceState{}, nil // missing is normal on first run
	}

	var ws WorkspaceState
	if err := json.Unmarshal(data, &ws); err != nil {
		return WorkspaceState{}, nil // corrupt cache — treat as empty
	}
	return ws, nil
}

// SaveState writes state to ~/.ergo/state/<state.Workspace>.json.
func SaveState(state WorkspaceState) error {
	path, err := statePath(state.Workspace)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling workspace state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing workspace state: %w", err)
	}
	return nil
}

// statePath returns the absolute path to the state file for wsName.
func statePath(wsName string) (string, error) {
	stateDir, err := config.ExpandTilde("~/.ergo/state")
	if err != nil {
		return "", fmt.Errorf("expanding state directory path: %w", err)
	}
	return filepath.Join(stateDir, wsName+".json"), nil
}
