package config

import "strings"

// GlobalConfig is the parsed representation of ~/.ergo/config.toml.
type GlobalConfig struct {
	Defaults DefaultsConfig `toml:"defaults"`
	Parallel ParallelConfig `toml:"parallel"`
	Sync     SyncConfig     `toml:"sync"`
	Run      RunConfig      `toml:"run"`
	Git      GitConfig      `toml:"git"`
}

// DefaultsConfig holds workspace and branch defaults.
type DefaultsConfig struct {
	WorkspaceRoot string `toml:"workspace_root"`
	DefaultBranch string `toml:"default_branch"`
}

// ParallelConfig controls parallel operation behavior.
type ParallelConfig struct {
	Enabled   bool `toml:"enabled"`
	BatchSize int  `toml:"batch_size"`
}

// SyncConfig controls sync behavior.
type SyncConfig struct {
	AutoPull bool `toml:"auto_pull"`
}

// RunConfig controls ergo run behavior.
type RunConfig struct {
	ExcludedGroups []string `toml:"excluded_groups"`
}

// Accepted values for [git].protocol.
const (
	GitProtocolHTTPS = "https"
	GitProtocolSSH   = "ssh"
)

// GitConfig controls how ergo invokes git.
type GitConfig struct {
	// Protocol selects the transport for ergo-initiated clones: "https" uses
	// repo URLs exactly as written in the workspace TOML; "ssh" rewrites
	// https:// URLs to scp form (git@host:owner/repo.git) in memory at clone
	// time only — the stored TOML is never modified.
	//
	// DECISION: "https" (and unset) means "leave URLs untouched" rather than
	// rewriting scp→https: that conversion is lossy (SSH host aliases from
	// ~/.ssh/config, non-standard users, and port mappings cannot be reliably
	// converted), and no-rewrite guarantees zero behavior change on upgrade.
	Protocol string `toml:"protocol"`
}

// UseSSH reports whether repo URLs should be rewritten to SSH form.
//
// DECISION: an empty Protocol means "https" (no rewriting). Existing config
// files decode into a zero-value GlobalConfig, so users without a [git]
// section get "" here — treating that as https downstream keeps backward
// compatibility without seeding the decode target with defaults (which would
// silently change other absent settings).
func (g GitConfig) UseSSH() bool {
	return strings.EqualFold(strings.TrimSpace(g.Protocol), GitProtocolSSH)
}

// WorkspaceConfig is the parsed representation of ~/.ergo/workspaces/<name>.toml.
type WorkspaceConfig struct {
	Workspace WorkspaceMeta `toml:"workspace"`
	Repos     []Repo        `toml:"repos"`
	Folders   []Folder      `toml:"folders"`
}

// WorkspaceMeta holds workspace-level metadata.
type WorkspaceMeta struct {
	Name   string        `toml:"name"`
	VSCode VSCodeSection `toml:"vscode"`
}

// VSCodeSection holds the workspace-level VS Code settings block.
type VSCodeSection struct {
	Settings map[string]any `toml:"settings"`
}

// Repo represents a single [[repos]] entry.
//
// Name is a pointer so we can distinguish "unset" (nil) from explicitly set,
// which triggers DeriveRepoName when nil.
// Branch is a pointer so we can detect when it was not specified and fall back
// to GlobalConfig.Defaults.DefaultBranch.
type Repo struct {
	URL            string         `toml:"url"`
	Name           *string        `toml:"name"`
	Branch         *string        `toml:"branch"`
	Tags           []string       `toml:"tags"`
	Group          string         `toml:"group"`
	VSCodeSettings map[string]any `toml:"vscode_settings"`
}

// EffectiveName returns the explicit name if set, otherwise the value derived
// from the URL. Returns an empty string if the URL is also empty.
func (r *Repo) EffectiveName() string {
	if r.Name != nil {
		return *r.Name
	}
	return DeriveRepoName(r.URL)
}

// Folder represents a single [[folders]] entry.
type Folder struct {
	Name           string         `toml:"name"`
	Git            bool           `toml:"git"`
	VSCodeSettings map[string]any `toml:"vscode_settings"`
}
