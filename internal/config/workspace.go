package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// LoadWorkspace loads ~/.ergo/workspaces/<name>.toml and returns the parsed config.
func LoadWorkspace(name string) (WorkspaceConfig, error) {
	home, err := ergoHome()
	if err != nil {
		return WorkspaceConfig{}, err
	}

	path := filepath.Join(home, "workspaces", name+".toml")
	var cfg WorkspaceConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return WorkspaceConfig{}, fmt.Errorf("parsing workspace config %s: %w", path, err)
	}
	return cfg, nil
}

// WriteWorkspace serializes cfg as TOML and writes it to
// ~/.ergo/workspaces/<name>.toml, creating any necessary parent directories.
func WriteWorkspace(name string, cfg WorkspaceConfig) error {
	home, err := ergoHome()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, "workspaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating workspaces directory: %w", err)
	}

	path := filepath.Join(dir, name+".toml")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("opening workspace config for write: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encoding workspace config: %w", err)
	}
	return nil
}

// ListWorkspaceNames returns all workspace names found in ~/.ergo/workspaces/.
func ListWorkspaceNames() ([]string, error) {
	home, err := ergoHome()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(home, "workspaces")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading workspaces directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
		}
	}
	return names, nil
}

// DeriveRepoName derives a directory name from a git clone URL.
// "https://github.com/juan/handwriting-recognition.git" → "handwriting-recognition"
// "git@github.com:juan/repo.git" → "repo"
func DeriveRepoName(url string) string {
	if url == "" {
		return ""
	}
	// Strip trailing slash before .git to handle "repo.git/" edge case.
	u := strings.TrimRight(url, "/")
	// Strip trailing .git
	u = strings.TrimSuffix(u, ".git")
	// Take the last path component (works for both https and scp-style git URLs)
	if idx := strings.LastIndexAny(u, "/:"); idx >= 0 {
		u = u[idx+1:]
	}
	return u
}

// ExpandTilde replaces a leading "~" with the user's home directory.
func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}
