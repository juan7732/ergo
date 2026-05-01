package vscode

import (
	"encoding/json"
	"fmt"

	"juan7732/ergo/internal/config"
)

// Filter holds the active view filter recorded in the "ergo" object of a
// generated .code-workspace file. All fields are optional — a zero-value
// Filter (or nil pointer) means no filter is active.
type Filter struct {
	Group string   `json:"group,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	Name  string   `json:"name,omitempty"`
}

// ergoMeta is the value of the top-level "ergo" key in the generated file.
// VS Code ignores unknown top-level keys; ergo uses this to identify its
// own files and carry filter state.
type ergoMeta struct {
	WorkspaceName string  `json:"workspace-name"`
	Filter        *Filter `json:"filter,omitempty"`
}

// wsFolder mirrors a VS Code folder entry.
type wsFolder struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Settings map[string]any `json:"settings,omitempty"`
}

// codeWorkspace is the full .code-workspace JSON structure.
type codeWorkspace struct {
	Ergo     ergoMeta       `json:"ergo"`
	Folders  []wsFolder     `json:"folders"`
	Settings map[string]any `json:"settings,omitempty"`
}

// Generate builds the .code-workspace JSON bytes from cfg.
// filter may be nil when no view filter is active.
//
// Generation rules (spec §5):
//   - Root folder {"name":"root","path":"."} is always first. Non-negotiable.
//   - Repos appear next, in TOML declaration order.
//   - Folders appear after all repos.
//   - Workspace-level vscode.settings → top-level "settings".
//   - Per-repo/folder vscode_settings → folder-level "settings".
func Generate(cfg config.WorkspaceConfig, filter *Filter) ([]byte, error) {
	ws := codeWorkspace{
		Ergo: ergoMeta{
			WorkspaceName: cfg.Workspace.Name,
			Filter:        filter,
		},
	}

	// Root folder — always first, non-negotiable (spec §5).
	ws.Folders = append(ws.Folders, wsFolder{Name: "root", Path: "."})

	// Repos in TOML declaration order.
	for i, repo := range cfg.Repos {
		name := repo.EffectiveName()
		if name == "" {
			return nil, fmt.Errorf("repos[%d]: cannot determine folder name (url is empty and no explicit name set)", i)
		}
		ws.Folders = append(ws.Folders, wsFolder{
			Name:     name,
			Path:     name,
			Settings: nilIfEmpty(repo.VSCodeSettings),
		})
	}

	// Non-repo folders after all repos.
	for _, dir := range cfg.Folders {
		ws.Folders = append(ws.Folders, wsFolder{
			Name:     dir.Name,
			Path:     dir.Name,
			Settings: nilIfEmpty(dir.VSCodeSettings),
		})
	}

	// Workspace-level VS Code settings.
	ws.Settings = nilIfEmpty(cfg.Workspace.VSCode.Settings)

	b, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling .code-workspace: %w", err)
	}
	// Append a trailing newline so the file is POSIX-compliant and diff-friendly.
	return append(b, '\n'), nil
}

// nilIfEmpty returns nil when m is nil or empty, otherwise returns m.
// This prevents emitting empty "settings": {} objects in the JSON output.
func nilIfEmpty(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}
