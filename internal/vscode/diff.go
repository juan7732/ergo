package vscode

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// WriteIfChanged writes content to path only when the file does not already
// contain the same bytes, avoiding unnecessary file-change events in VS Code
// (spec §5 — smart regeneration).
//
// Returns true when the file was written, false when it was already up to date.
func WriteIfChanged(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("creating directory for %s: %w", path, err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}
