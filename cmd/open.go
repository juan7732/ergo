package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/vscode"
	"juan7732/ergo/internal/workspace"
)

var openCmd = &cobra.Command{
	Use:   "open [workspace-name]",
	Short: "Open a workspace in VS Code",
	Long: `Open a workspace in VS Code, creating it on disk if it doesn't exist yet.

First-time use: clones all repos, creates folders, generates the .code-workspace
file, then launches VS Code.

Subsequent use: regenerates .code-workspace only if the content has changed
(smart regeneration), then launches VS Code. Does not re-clone or pull — use
'ergo sync' for that.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	wsFilePath := filepath.Join(wsDir, name+".code-workspace")

	// Check for the fast path: workspace dir exists and .code-workspace is current.
	if isWorkspaceCurrent(wsDir, wsFilePath, wsCfg) {
		fmt.Fprintf(cmd.OutOrStdout(), "opening %s\n", wsFilePath)
		return launchVSCode(wsFilePath)
	}

	// Workspace directory is missing or the .code-workspace file is stale.
	// REVIEW: if the workspace dir exists but the TOML has new repos since last
	// sync, isWorkspaceCurrent returns false (generated content differs), so we
	// regenerate the .code-workspace but do NOT clone the new repos. The spec
	// says open syncs when the directory is missing but is silent about the
	// "dir exists, TOML changed" case. Current behavior — regenerate the file,
	// leave cloning to 'ergo sync' — feels correct: open is for opening.
	dirExists := dirExistsOnDisk(wsDir)
	if !dirExists {
		// First-time materialization: clone repos, create folders.
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "creating workspace %q → %s\n\n", name, wsDir)

		opts := workspace.SyncOptions{
			WorkspaceDir: wsDir,
			AutoPull:     false, // new workspace — only clone, no pull needed
			Parallel:     globalCfg.Parallel.Enabled,
			BatchSize:    globalCfg.Parallel.BatchSize,
			Progress: func(repoName string, action workspace.RepoAction, syncErr error) {
				if syncErr != nil {
					fmt.Fprintf(out, "  ✗ %-30s %s\n", repoName, syncErr)
				} else {
					fmt.Fprintf(out, "  ✓ %-30s %s\n", repoName, action)
				}
			},
		}

		result, syncErr := workspace.Sync(wsCfg, opts, execRunner())
		if syncErr != nil {
			return fmt.Errorf("syncing workspace: %w", syncErr)
		}

		// Print folder results.
		for _, fr := range result.Folders {
			if fr.Err != nil {
				fmt.Fprintf(out, "  ✗ %-30s %s\n", fr.Name+"/", fr.Err)
			} else if fr.Action != workspace.FolderActionSkipped {
				fmt.Fprintf(out, "  ✓ %-30s %s\n", fr.Name+"/", fr.Action)
			}
		}
		fmt.Fprintln(out)
	}

	// Regenerate .code-workspace (smart — only write if changed).
	wsBytes, err := vscode.Generate(wsCfg, nil)
	if err != nil {
		return fmt.Errorf("generating .code-workspace: %w", err)
	}
	if _, err := vscode.WriteIfChanged(wsFilePath, wsBytes); err != nil {
		return fmt.Errorf("writing .code-workspace: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "opening %s\n", wsFilePath)
	return launchVSCode(wsFilePath)
}

// isWorkspaceCurrent returns true when the workspace directory exists on disk
// and the existing .code-workspace file matches what would be generated from wsCfg.
// Returns false on any I/O error (conservative: triggers regeneration).
func isWorkspaceCurrent(wsDir, wsFilePath string, wsCfg config.WorkspaceConfig) bool {
	if !dirExistsOnDisk(wsDir) {
		return false
	}
	existing, err := os.ReadFile(wsFilePath)
	if err != nil {
		return false
	}
	expected, err := vscode.Generate(wsCfg, nil)
	if err != nil {
		return false
	}
	return bytes.Equal(existing, expected)
}

// dirExistsOnDisk returns true when path is an existing directory.
func dirExistsOnDisk(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// launchVSCode execs the `code` binary with the given path.
// It checks that `code` is on PATH before attempting to launch.
func launchVSCode(wsFilePath string) error {
	codePath, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("'code' not found on PATH: install the VS Code CLI and retry (https://code.visualstudio.com/docs/setup/mac)")
	}
	cmd := exec.Command(codePath, wsFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launching VS Code: %w", err)
	}
	return nil
}
