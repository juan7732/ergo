package vscode

import (
	"encoding/json"
	"fmt"
	"os"
)

// IsZero reports whether no filter dimension is set. A recorded filter object
// with no fields is equivalent to no filter at all.
func (f Filter) IsZero() bool {
	return f.Group == "" && len(f.Tags) == 0 && f.Name == ""
}

// ReadFilter reads the view filter recorded in the "ergo" object of an
// existing .code-workspace file — the counterpart to Generate, which writes
// it. Returns nil when the file records no filter (or an empty one).
//
// Unknown fields anywhere in the document are tolerated: the file is shared
// with VS Code and future ergo versions, and readers must stay additive.
//
// Errors (missing file, unreadable, malformed JSON) are returned for the
// caller to decide; filter-preservation callers treat any error as "no
// filter" so that regeneration never fails over filter recovery.
func ReadFilter(path string) (*Filter, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var doc struct {
		Ergo struct {
			Filter *Filter `json:"filter"`
		} `json:"ergo"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if doc.Ergo.Filter == nil || doc.Ergo.Filter.IsZero() {
		return nil, nil
	}
	return doc.Ergo.Filter, nil
}

// Describe renders the filter for human-facing messages, e.g. `group "ml"`
// or `tags "go", "ml"`. Multiple active dimensions are comma-separated in
// group, tags, name order (matching ApplyRepoFilter's AND semantics).
func (f Filter) Describe() string {
	var parts []string
	if f.Group != "" {
		parts = append(parts, fmt.Sprintf("group %q", f.Group))
	}
	if len(f.Tags) > 0 {
		tags := ""
		for i, t := range f.Tags {
			if i > 0 {
				tags += ", "
			}
			tags += fmt.Sprintf("%q", t)
		}
		parts = append(parts, "tags "+tags)
	}
	if f.Name != "" {
		parts = append(parts, fmt.Sprintf("name %q", f.Name))
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
