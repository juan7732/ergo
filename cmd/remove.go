package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/tui"
)

var removeCmd = &cobra.Command{
	Use:   "remove [workspace-name]",
	Short: "Remove a repo or folder from a workspace",
	Long: `Remove one or more repos or folders from a workspace definition.

Without subcommands, launches a multi-select TUI.
Use 'repo' or 'folder' subcommands for non-interactive shorthand.

By default, only the TOML entry is removed — files on disk are untouched.
Use --force to also delete the directory from disk (requires confirmation).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRemove,
}

var removeRepoCmd = &cobra.Command{
	Use:   "repo <name>",
	Short: "Remove a repo from the current workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemoveRepo,
}

var removeFolderCmd = &cobra.Command{
	Use:   "folder <name>",
	Short: "Remove a folder from the current workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemoveFolder,
}

func init() {
	removeCmd.Flags().Bool("force", false, "Also delete the directory from disk (requires confirmation)")
	removeRepoCmd.Flags().Bool("force", false, "Also delete the directory from disk (requires confirmation)")
	removeFolderCmd.Flags().Bool("force", false, "Also delete the directory from disk (requires confirmation)")

	removeCmd.AddCommand(removeRepoCmd)
	removeCmd.AddCommand(removeFolderCmd)
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	items := make([]tui.RemoveItem, 0, len(wsCfg.Repos)+len(wsCfg.Folders))
	for _, r := range wsCfg.Repos {
		items = append(items, tui.RemoveItem{Name: r.EffectiveName(), IsRepo: true})
	}
	for _, f := range wsCfg.Folders {
		items = append(items, tui.RemoveItem{Name: f.Name, IsRepo: false})
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "workspace has no repos or folders to remove")
		return nil
	}

	sel := tui.NewRemoveSelect(items)
	finalModel, err := tui.RunInline(sel)
	if err != nil {
		return fmt.Errorf("running remove selector: %w", err)
	}

	selected, ok := finalModel.(tui.RemoveSelect).Result()
	if !ok || len(selected) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
		return nil
	}

	return applyRemoval(cmd, name, wsCfg, selected, wsDir, force)
}

func runRemoveRepo(cmd *cobra.Command, args []string) error {
	repoName := args[0]
	force, _ := cmd.Flags().GetBool("force")

	name, err := resolveWorkspaceName(cmd, "")
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	found := false
	for _, r := range wsCfg.Repos {
		if r.EffectiveName() == repoName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("repo %q not found in workspace %q", repoName, name)
	}

	return applyRemoval(cmd, name, wsCfg,
		[]tui.RemoveItem{{Name: repoName, IsRepo: true}},
		wsDir, force)
}

func runRemoveFolder(cmd *cobra.Command, args []string) error {
	folderName := args[0]
	force, _ := cmd.Flags().GetBool("force")

	name, err := resolveWorkspaceName(cmd, "")
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	found := false
	for _, f := range wsCfg.Folders {
		if f.Name == folderName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("folder %q not found in workspace %q", folderName, name)
	}

	return applyRemoval(cmd, name, wsCfg,
		[]tui.RemoveItem{{Name: folderName, IsRepo: false}},
		wsDir, force)
}

// applyRemoval removes the selected items from the TOML and optionally deletes
// their directories from disk when force is true.
func applyRemoval(cmd *cobra.Command, wsName string, wsCfg config.WorkspaceConfig, items []tui.RemoveItem, wsDir string, force bool) error {
	out := cmd.OutOrStdout()

	if force {
		fmt.Fprintln(out, "the following directories will be deleted from disk:")
		for _, item := range items {
			fmt.Fprintf(out, "  %s\n", filepath.Join(wsDir, item.Name))
		}
		fmt.Fprint(out, "confirm? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() || strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			fmt.Fprintln(out, "cancelled")
			return nil
		}
	}

	// Build lookup sets for items to remove.
	removeRepos := make(map[string]struct{}, len(items))
	removeFolders := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.IsRepo {
			removeRepos[item.Name] = struct{}{}
		} else {
			removeFolders[item.Name] = struct{}{}
		}
	}

	// Filter the workspace config in-place.
	updated := wsCfg
	updated.Repos = nil
	for _, r := range wsCfg.Repos {
		if _, remove := removeRepos[r.EffectiveName()]; !remove {
			updated.Repos = append(updated.Repos, r)
		}
	}
	updated.Folders = nil
	for _, f := range wsCfg.Folders {
		if _, remove := removeFolders[f.Name]; !remove {
			updated.Folders = append(updated.Folders, f)
		}
	}

	if err := config.WriteWorkspace(wsName, updated); err != nil {
		return fmt.Errorf("writing workspace config: %w", err)
	}

	for _, item := range items {
		if force {
			dir := filepath.Join(wsDir, item.Name)
			if err := os.RemoveAll(dir); err != nil {
				fmt.Fprintf(out, "  ✗ deleted %s: %v\n", dir, err)
			} else {
				fmt.Fprintf(out, "  ✓ deleted %s\n", dir)
			}
		} else {
			fmt.Fprintf(out, "  removed %q from TOML (directory left on disk)\n", item.Name)
		}
	}
	return nil
}
