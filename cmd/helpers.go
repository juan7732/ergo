package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/git"
)

// isTerminal reports whether stdin is an interactive terminal.
// Used to decide whether to show interactive prompts.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// currentDir returns the current working directory.
func currentDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return cwd, nil
}

// workspaceDir returns the absolute path to the workspace directory on disk
// for the given workspace name, expanding ~ in the configured workspace root.
func workspaceDir(globalCfg config.GlobalConfig, name string) (string, error) {
	wsRoot, err := config.ExpandTilde(globalCfg.Defaults.WorkspaceRoot)
	if err != nil {
		return "", fmt.Errorf("expanding workspace root: %w", err)
	}
	return filepath.Join(wsRoot, name), nil
}

// execRunner returns the default git runner that shells out to the real git binary.
func execRunner() git.Runner {
	return git.ExecRunner{}
}
