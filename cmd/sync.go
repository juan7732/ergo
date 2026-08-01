package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/vscode"
	"github.com/juan7732/ergo/internal/workspace"
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
	syncCmd.Flags().String("name", "", "Filter repos by name (glob pattern, case-insensitive)")
	syncCmd.Flags().String("group", "", "Filter repos to this group")
	syncCmd.Flags().StringSlice("tags", nil, "Filter repos by tags, any-match (comma-separated or repeated flag)")
	// --force deletes orphans, --add adopts them: contradictory in one run, and
	// running both would delete dirs the --add step then tries to re-adopt.
	syncCmd.MarkFlagsMutuallyExclusive("force", "add")
	rootCmd.AddCommand(syncCmd)
}

// syncParams captures everything executeSync needs beyond the resolved
// workspace name. The zero value means "sync everything, delete nothing".
type syncParams struct {
	Force  bool
	Add    bool
	Filter workspace.FilterOptions
}

// syncRunner is the seam executeSync is reached through, so callers like
// promptSync can be tested without spawning a real sync.
var syncRunner = executeSync

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

	filterOpts, err := filterOptsFromFlags(cmd, nil)
	if err != nil {
		return err
	}

	return syncRunner(cmd, name, syncParams{Force: force, Add: add, Filter: filterOpts})
}

func executeSync(cmd *cobra.Command, name string, p syncParams) error {
	force, add := p.Force, p.Add

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	// Apply filter flags to restrict which repos are synced. Orphan detection,
	// however, always runs against the full config (knownNames below) so that
	// out-of-filter repos are never mistaken for orphans.
	filteredRepos := workspace.ApplyRepoFilter(wsCfg.Repos, p.Filter)
	syncCfg := wsCfg
	syncCfg.Repos = filteredRepos

	knownNames := make([]string, 0, len(wsCfg.Repos)+len(wsCfg.Folders))
	for _, r := range wsCfg.Repos {
		knownNames = append(knownNames, r.EffectiveName())
	}
	for _, f := range wsCfg.Folders {
		knownNames = append(knownNames, f.Name)
	}

	fmt.Fprintf(out, "syncing workspace %q → %s\n\n", name, wsDir)

	opts := workspace.SyncOptions{
		WorkspaceDir:     wsDir,
		AutoPull:         globalCfg.Sync.AutoPull,
		Parallel:         globalCfg.Parallel.Enabled,
		BatchSize:        globalCfg.Parallel.BatchSize,
		RewriteURLsToSSH: globalCfg.Git.UseSSH(),
		KnownNames:       knownNames,
		Progress: func(repoName string, action workspace.RepoAction, syncErr error) {
			if syncErr != nil {
				fmt.Fprintf(out, "  ✗ %-30s %s\n", repoName, syncErr)
			} else {
				fmt.Fprintf(out, "  ✓ %-30s %s\n", repoName, action)
			}
		},
	}

	result, err := workspace.Sync(syncCfg, opts, execRunner())
	if err != nil {
		return fmt.Errorf("syncing workspace: %w", err)
	}

	// Persist state cache for repos that were cloned or pulled.
	state := workspace.WorkspaceState{
		Workspace: name,
		LastSync:  time.Now(),
		Repos:     make(map[string]workspace.RepoStateEntry),
	}
	for _, rr := range result.Repos {
		if rr.Action == workspace.RepoActionCloned || rr.Action == workspace.RepoActionPulled {
			state.Repos[rr.Name] = workspace.RepoStateEntry{LastPulled: time.Now()}
		}
	}
	if saveErr := workspace.SaveState(state); saveErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: saving state: %v\n", saveErr)
	}

	// Print folder results.
	for _, fr := range result.Folders {
		if fr.Err != nil {
			fmt.Fprintf(out, "  ✗ %-30s %s\n", fr.Name+"/", fr.Err)
		} else if fr.Action != workspace.FolderActionSkipped {
			fmt.Fprintf(out, "  ✓ %-30s %s\n", fr.Name+"/", fr.Action)
		}
	}

	// Regenerate .code-workspace, preserving any active show filter recorded
	// in the existing file (ergo-vscode-spec.md §3.2). A missing or malformed
	// file degrades to nil — regenerate unfiltered, never fail sync over
	// filter recovery.
	//
	// DECISION: the show filter is purely a view concern. Sync always
	// operates on the full TOML — repos hidden by the filter were still
	// cloned/pulled above; only the folders list of the generated file (and
	// the recorded ergo.filter) reflect it. The operation set is governed
	// solely by the explicit --name/--group/--tags flags.
	wsFilePath := filepath.Join(wsDir, name+".code-workspace")
	viewFilter := preservedFilter(wsFilePath)
	wsBytes, err := generateView(wsCfg, viewFilter)
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
	if viewFilter != nil {
		visible := len(workspace.ApplyRepoFilter(wsCfg.Repos, showFilterOptions(viewFilter)))
		fmt.Fprintln(out, filterNote(viewFilter, visible, len(wsCfg.Repos)))
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
