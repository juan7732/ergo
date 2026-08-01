package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/git"
	"github.com/juan7732/ergo/internal/output"
	"github.com/juan7732/ergo/internal/vscode"
	"github.com/juan7732/ergo/internal/workspace"
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

// filterOptsFromFlags builds a FilterOptions from the standard --name, --group,
// and --tags flags registered on cmd. excludedGroups comes from the global
// config's [run].excluded_groups and is stored in FilterOptions for callers that
// need it; callers that do not apply exclusion may leave it nil.
//
// Requires --name, --group, and --tags flags to be registered on cmd.
func filterOptsFromFlags(cmd *cobra.Command, excludedGroups []string) (workspace.FilterOptions, error) {
	name, _ := cmd.Flags().GetString("name")
	group, _ := cmd.Flags().GetString("group")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	return workspace.FilterOptions{
		Name:           name,
		Group:          group,
		Tags:           tags,
		ExcludedGroups: excludedGroups,
	}, nil
}

// printJSON marshals doc through the output package and writes the single
// JSON document to cmd's stdout. --json code paths must write nothing else
// to stdout (warnings go to stderr as plain text).
func printJSON(cmd *cobra.Command, doc any) error {
	b, err := output.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(b)
	return err
}

// preservedFilter reads the active show filter recorded in the workspace
// file at wsFilePath. Any error — file missing, unreadable, malformed JSON —
// degrades to nil: filter recovery must never fail the calling operation
// (ergo-vscode-spec.md §3.2), which then regenerates unfiltered, matching
// pre-preservation behavior.
func preservedFilter(wsFilePath string) *vscode.Filter {
	f, err := vscode.ReadFilter(wsFilePath)
	if err != nil {
		return nil
	}
	return f
}

// showFilterOptions converts a preserved view filter into the FilterOptions
// equivalent that ApplyRepoFilter understands.
func showFilterOptions(f *vscode.Filter) workspace.FilterOptions {
	if f == nil {
		return workspace.FilterOptions{}
	}
	return workspace.FilterOptions{Name: f.Name, Group: f.Group, Tags: f.Tags}
}

// generateView renders the .code-workspace bytes for cfg as seen through an
// optional preserved show filter: the folders list is filtered and the
// filter stays recorded in the ergo object. A nil filter renders the full
// view. Shared by sync and open so both preserve filters identically.
func generateView(cfg config.WorkspaceConfig, f *vscode.Filter) ([]byte, error) {
	if f == nil {
		return vscode.Generate(cfg, nil)
	}
	viewCfg := cfg
	viewCfg.Repos = workspace.ApplyRepoFilter(cfg.Repos, showFilterOptions(f))
	return vscode.Generate(viewCfg, f)
}

// filterNote is the one-line notice sync, open, and human-format status
// print when a preserved show filter is active, so a filtered view is never
// silently in effect.
func filterNote(f *vscode.Filter, visible, total int) string {
	return fmt.Sprintf("note: show filter active (%s) — %d of %d repos visible; run 'ergo show all' to clear",
		f.Describe(), visible, total)
}
