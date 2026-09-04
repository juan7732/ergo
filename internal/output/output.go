// Package output defines the JSON documents emitted by the --json flag on
// status, list, validate, show, config, and search.
//
// These schemas are a stable wire contract consumed by external tooling (the
// ergo VS Code extension pins its minimum ergo version against them). Field
// names never change or disappear once shipped; evolution is additive only.
// Consumers must tolerate unknown fields.
//
// DECISION: the wire format lives in dedicated structs here rather than as
// json tags on the internal types (workspace.RepoStatusEntry, vscode.Filter,
// config.ValidationError), so internal refactors cannot silently change the
// contract. Every struct is converted explicitly via a constructor.
package output

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/vscode"
	"github.com/juan7732/ergo/internal/workspace"
)

// Marshal renders v as the single JSON document a --json command prints on
// stdout: two-space indented with a trailing newline (matching the generated
// .code-workspace style), no ANSI, no surrounding prose.
func Marshal(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling json output: %w", err)
	}
	return append(b, '\n'), nil
}

// ─── ergo status --json ───────────────────────────────────────────────────────

// StatusRepo is one repo entry in the status document.
// Uncloned repos report branch "", dirty false, behind 0.
type StatusRepo struct {
	Name     string   `json:"name"`
	Branch   string   `json:"branch"`
	Dirty    bool     `json:"dirty"`
	Behind   int      `json:"behind"`
	Uncloned bool     `json:"uncloned"`
	Group    string   `json:"group"`
	Tags     []string `json:"tags"`
}

// Status is the document printed by `ergo status [ws] --json`.
//
// In standalone-repo mode (status run outside any ergo workspace, inside a
// plain git repo) the same shape is emitted with workspace set to the empty
// string and a single repo entry whose group is "" and tags are [].
type Status struct {
	Workspace string       `json:"workspace"`
	Repos     []StatusRepo `json:"repos"`
}

// NewStatus builds the status document from gathered entries.
func NewStatus(workspaceName string, entries []workspace.RepoStatusEntry) Status {
	repos := make([]StatusRepo, 0, len(entries))
	for _, e := range entries {
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		repos = append(repos, StatusRepo{
			Name:     e.Name,
			Branch:   e.Branch,
			Dirty:    e.Dirty,
			Behind:   e.Behind,
			Uncloned: e.Uncloned,
			Group:    e.Group,
			Tags:     tags,
		})
	}
	return Status{Workspace: workspaceName, Repos: repos}
}

// ─── ergo list --json ─────────────────────────────────────────────────────────

// ListWorkspace is one workspace entry in the list document.
// Synced mirrors the human table: true when the workspace directory exists
// on disk, false when the TOML has not been materialized yet.
type ListWorkspace struct {
	Name   string `json:"name"`
	Repos  int    `json:"repos"`
	Synced bool   `json:"synced"`
}

// List is the document printed by `ergo list --json`.
// With no workspaces defined it is {"workspaces": []}, exit 0.
type List struct {
	Workspaces []ListWorkspace `json:"workspaces"`
}

// NewList builds the list document, normalizing a nil slice to [].
func NewList(workspaces []ListWorkspace) List {
	if workspaces == nil {
		workspaces = []ListWorkspace{}
	}
	return List{Workspaces: workspaces}
}

// ─── ergo validate --json ─────────────────────────────────────────────────────

// ValidationIssue is one problem found in a workspace config.
type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validate is the document printed by `ergo validate [ws] --json`.
// Errors is always present: [] when valid.
type Validate struct {
	Workspace string            `json:"workspace"`
	Valid     bool              `json:"valid"`
	Errors    []ValidationIssue `json:"errors"`
}

// ValidateAll is the document printed by `ergo validate --all --json`.
type ValidateAll struct {
	Workspaces []Validate `json:"workspaces"`
}

// NewValidate builds the validate document for one workspace from the error
// returned by loading + validating it. A nil err means valid.
//
// DECISION: a workspace whose TOML fails to parse (vs. failing validation)
// is represented as valid=false with a single error whose field is "" and
// whose message is the parse error — same shape, no extra field needed, and
// consumers already have to handle field-less errors.
func NewValidate(workspaceName string, err error) Validate {
	doc := Validate{Workspace: workspaceName, Valid: err == nil, Errors: []ValidationIssue{}}
	if err == nil {
		return doc
	}

	var ve config.ValidationErrors
	if errors.As(err, &ve) {
		for _, e := range ve {
			doc.Errors = append(doc.Errors, ValidationIssue{Field: e.Field, Message: e.Message})
		}
		return doc
	}

	doc.Errors = append(doc.Errors, ValidationIssue{Field: "", Message: err.Error()})
	return doc
}

// NewValidateAll builds the --all document, normalizing a nil slice to [].
func NewValidateAll(workspaces []Validate) ValidateAll {
	if workspaces == nil {
		workspaces = []Validate{}
	}
	return ValidateAll{Workspaces: workspaces}
}

// ─── ergo config --json ───────────────────────────────────────────────────────

// ConfigRepo is one [[repos]] entry in the config document.
// Name is the effective name (explicit or URL-derived) — the normalization
// consumers cannot do without reimplementing DeriveRepoName.
type ConfigRepo struct {
	Name  string   `json:"name"`
	URL   string   `json:"url"`
	Tags  []string `json:"tags"`
	Group string   `json:"group"`
}

// ConfigFolder is one [[folders]] entry in the config document.
type ConfigFolder struct {
	Name string `json:"name"`
	Git  bool   `json:"git"`
}

// Config is the document printed by `ergo config [ws] --json`: the workspace
// TOML normalized to JSON.
//
// DECISION: deliberately minimal — effective name, url, tags, group per repo
// and name, git per folder. branch and vscode_settings are omitted until a
// consumer needs them; the contract is additive, so adding costs nothing
// later while shipping them now would be a permanent commitment.
type Config struct {
	Workspace string         `json:"workspace"`
	Repos     []ConfigRepo   `json:"repos"`
	Folders   []ConfigFolder `json:"folders"`
}

// NewConfig builds the config document from a parsed workspace TOML.
func NewConfig(workspaceName string, cfg config.WorkspaceConfig) Config {
	doc := Config{
		Workspace: workspaceName,
		Repos:     make([]ConfigRepo, 0, len(cfg.Repos)),
		Folders:   make([]ConfigFolder, 0, len(cfg.Folders)),
	}
	for _, r := range cfg.Repos {
		tags := r.Tags
		if tags == nil {
			tags = []string{}
		}
		doc.Repos = append(doc.Repos, ConfigRepo{
			Name:  r.EffectiveName(),
			URL:   r.URL,
			Tags:  tags,
			Group: r.Group,
		})
	}
	for _, f := range cfg.Folders {
		doc.Folders = append(doc.Folders, ConfigFolder{Name: f.Name, Git: f.Git})
	}
	return doc
}

// ─── ergo show --json ─────────────────────────────────────────────────────────

// ShowFilter mirrors the ergo.filter object recorded in the .code-workspace.
//
// DECISION: sub-fields use omitempty so the document matches the stored
// filter byte-for-byte in spirit ({"group": "ml"}, {"tags": [...]}, or a
// combination) — consumers detect the filter kind by key presence, exactly
// as they would reading the .code-workspace directly.
type ShowFilter struct {
	Name  string   `json:"name,omitempty"`
	Group string   `json:"group,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// Show is the document printed by `ergo show --json` (read-only mode).
// Filter is null when no filter is active.
type Show struct {
	Workspace string      `json:"workspace"`
	Filter    *ShowFilter `json:"filter"`
}

// NewShow builds the show document from the filter read out of the
// .code-workspace file. f may be nil (no active filter).
func NewShow(workspaceName string, f *vscode.Filter) Show {
	doc := Show{Workspace: workspaceName}
	if f != nil {
		doc.Filter = &ShowFilter{Name: f.Name, Group: f.Group, Tags: f.Tags}
	}
	return doc
}

// ─── ergo search --json ───────────────────────────────────────────────────────

// SearchResult is one hit in the search document. Kind ("repo", "folder",
// or "workspace") discriminates the entry, and the wire shape varies by
// kind: url, group, tags, and cloned appear only for repos; created only
// for folders; synced only for workspaces. Fields that exist for a kind are
// always emitted (tags is [] when unset, group is "" when unset). Path is
// the absolute on-disk location whether or not it exists yet.
//
// DECISION: the per-kind shape is produced by MarshalJSON switching over
// three unexported wire structs rather than by omitempty on pointer fields.
// omitempty cannot express "tags: [] always present for repos, absent for
// folders", and pointer-typed fields would push nil checks onto every Go
// consumer for what is purely a wire concern.
type SearchResult struct {
	Workspace string
	Kind      string
	Name      string
	URL       string
	Group     string
	Tags      []string
	// Exists is the kind's on-disk bit: cloned for repos, created for
	// folders, synced for workspaces.
	Exists bool
	Path   string
}

type searchRepoWire struct {
	Workspace string   `json:"workspace"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Group     string   `json:"group"`
	Tags      []string `json:"tags"`
	Cloned    bool     `json:"cloned"`
	Path      string   `json:"path"`
}

type searchFolderWire struct {
	Workspace string `json:"workspace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Created   bool   `json:"created"`
	Path      string `json:"path"`
}

type searchWorkspaceWire struct {
	Workspace string `json:"workspace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Synced    bool   `json:"synced"`
	Path      string `json:"path"`
}

// MarshalJSON renders the kind-specific wire shape documented on SearchResult.
func (r SearchResult) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case string(workspace.HitKindRepo):
		tags := r.Tags
		if tags == nil {
			tags = []string{}
		}
		return json.Marshal(searchRepoWire{
			Workspace: r.Workspace, Kind: r.Kind, Name: r.Name,
			URL: r.URL, Group: r.Group, Tags: tags, Cloned: r.Exists, Path: r.Path,
		})
	case string(workspace.HitKindFolder):
		return json.Marshal(searchFolderWire{
			Workspace: r.Workspace, Kind: r.Kind, Name: r.Name, Created: r.Exists, Path: r.Path,
		})
	case string(workspace.HitKindWorkspace):
		return json.Marshal(searchWorkspaceWire{
			Workspace: r.Workspace, Kind: r.Kind, Name: r.Name, Synced: r.Exists, Path: r.Path,
		})
	}
	return nil, fmt.Errorf("marshaling search result: unknown kind %q", r.Kind)
}

// Search is the document printed by `ergo search <query> --json`.
// With no hits it is {"query": "...", "results": []}, exit 0.
type Search struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
}

// NewSearch builds the search document from matcher hits, normalizing a nil
// slice to [].
func NewSearch(query string, hits []workspace.Hit) Search {
	results := make([]SearchResult, 0, len(hits))
	for _, h := range hits {
		results = append(results, SearchResult{
			Workspace: h.Workspace,
			Kind:      string(h.Kind),
			Name:      h.Name,
			URL:       h.URL,
			Group:     h.Group,
			Tags:      h.Tags,
			Exists:    h.Exists,
			Path:      h.Path,
		})
	}
	return Search{Query: query, Results: results}
}
