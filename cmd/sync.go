package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/vscode"
	"juan7732/ergo/internal/workspace"
)

var syncCmd = &cobra.Command{
	Use:   "sync [workspace-name]",
	Short: "Synchronize workspace on disk with TOML configuration",
	Long: `Synchronize the workspace directory on disk with the TOML configuration.

For each repo:
  - If the directory doesn't exist, clones it.
  - If the directory exists and auto_pull is true, runs git pull.
  - If the directory exists and auto_pull is false, skips.

For each folder:
  - If the directory doesn't exist, creates it.
  - If git=true and not yet a git repo, runs git init.

Sync never deletes. Use --force to delete orphaned directories (with confirmation).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

func init() {
	syncCmd.Flags().Bool("force", false, "Delete directories that are not in the TOML (requires confirmation)")
	syncCmd.Flags().Bool("add", false, "Add repos/folders found on disk but not in TOML to the config (requires confirmation)")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	add, _ := cmd.Flags().GetBool("add")

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

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "syncing workspace %q → %s\n\n", name, wsDir)

	opts := workspace.SyncOptions{
		WorkspaceDir: wsDir,
		AutoPull:     globalCfg.Sync.AutoPull,
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

	result, err := workspace.Sync(wsCfg, opts, execRunner())
	if err != nil {
		return fmt.Errorf("syncing workspace: %w", err)
	}

	// Print folder results.
	for _, fr := range result.Folders {
		if fr.Err != nil {
			fmt.Fprintf(out, "  ✗ %-30s %s\n", fr.Name+"/", fr.Err)
		} else if fr.Action != workspace.FolderActionSkipped {
			fmt.Fprintf(out, "  ✓ %-30s %s\n", fr.Name+"/", fr.Action)
		}
	}

	// Regenerate .code-workspace.
	wsFilePath := filepath.Join(wsDir, name+".code-workspace")
	wsBytes, err := vscode.Generate(wsCfg, nil)
	if err != nil {
		return fmt.Errorf("generating .code-workspace: %w", err)
	}
	written, err := vscode.WriteIfChanged(wsFilePath, wsBytes)
	if err != nil {
		return fmt.Errorf("writing .code-workspace: %w", err)
	}
	fmt.Fprintln(out)
	if written {
		fmt.Fprintf(out, "  updated  %s\n", filepath.Base(wsFilePath))
	} else {
		fmt.Fprintf(out, "  unchanged  %s\n", filepath.Base(wsFilePath))
	}

	// Report orphans.
	if len(result.Orphans) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "orphaned directories (not in TOML):")
		for _, o := range result.Orphans {
			fmt.Fprintf(out, "  %s\n", o)
		}

		if force {
			if err := confirmAndDeleteOrphans(cmd, wsDir, result.Orphans); err != nil {
				return err
			}
		} else {
			fmt.Fprintln(out, "\nuse --force to delete them")
		}
	}

	// Handle --add: add on-disk repos/folders missing from TOML.
	if add {
		if err := addOrphansToConfig(cmd, name, wsDir, wsCfg, result.Orphans); err != nil {
			return err
		}
	}

	// Summary.
	fmt.Fprintln(out)
	errCount := 0
	for _, rr := range result.Repos {
		if rr.Err != nil {
			errCount++
		}
	}
	for _, fr := range result.Folders {
		if fr.Err != nil {
			errCount++
		}
	}
	if errCount > 0 {
		return fmt.Errorf("sync completed with %d error(s)", errCount)
	}
	fmt.Fprintln(out, "done")
	return nil
}

// confirmAndDeleteOrphans prompts the user to confirm deletion, then removes each orphan.
func confirmAndDeleteOrphans(cmd *cobra.Command, wsDir string, orphans []string) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\ndelete %d orphaned director(ies)? [y/N] ", len(orphans))

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(line)) != "y" {
		fmt.Fprintln(out, "skipped")
		return nil
	}

	for _, o := range orphans {
		if err := workspace.DeleteOrphan(wsDir, o); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %v\n", o, err)
		} else {
			fmt.Fprintf(out, "  deleted  %s\n", o)
		}
	}
	return nil
}

// addOrphansToConfig scans for git repos among the orphans and prompts to add them to the TOML.
func addOrphansToConfig(cmd *cobra.Command, wsName, wsDir string, wsCfg config.WorkspaceConfig, orphans []string) error {
	out := cmd.OutOrStdout()

	if len(orphans) == 0 {
		return nil
	}

	fmt.Fprintf(out, "\nadd %d director(ies) to TOML? [y/N] ", len(orphans))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(line)) != "y" {
		fmt.Fprintln(out, "skipped")
		return nil
	}

	r := execRunner()
	for _, name := range orphans {
		dir := filepath.Join(wsDir, name)
		// Check if it's a git repo by looking for a remote URL.
		url, gitErr := r.Run(dir, "git", "remote", "get-url", "origin")
		if gitErr == nil && strings.TrimSpace(url) != "" {
			// It's a git repo with a remote — add as repo.
			wsCfg.Repos = append(wsCfg.Repos, config.Repo{
				URL: strings.TrimSpace(url),
			})
			fmt.Fprintf(out, "  added repo  %s (%s)\n", name, strings.TrimSpace(url))
		} else {
			// Not a git repo or no remote — add as folder.
			wsCfg.Folders = append(wsCfg.Folders, config.Folder{Name: name})
			fmt.Fprintf(out, "  added folder  %s\n", name)
		}
	}

	if err := config.WriteWorkspace(wsName, wsCfg); err != nil {
		return fmt.Errorf("writing updated workspace config: %w", err)
	}
	return nil
}
