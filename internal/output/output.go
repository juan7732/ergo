// Package output defines the JSON documents emitted by the --json flag on
// status, list, validate, and show.
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
