package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/output"
	"github.com/juan7732/ergo/internal/tui"
	"github.com/juan7732/ergo/internal/workspace"
)

var statusCmd = &cobra.Command{
	Use:   "status [workspace-name]",
	Short: "Show the state of all repos in a workspace",
	Long: `Show branch, dirty state, and ahead/behind count for each repo.

When run inside a standalone git repo (outside any ergo workspace), shows
status for just that one repo.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolP("short", "s", false, "One-line-per-repo output, no table borders (for scripting)")
	statusCmd.Flags().Bool("json", false, "Print a machine-readable JSON document instead of the table")
	statusCmd.Flags().String("name", "", "Filter repos by name (glob pattern, case-insensitive)")
	statusCmd.Flags().String("group", "", "Filter repos to this group")
	statusCmd.Flags().StringSlice("tags", nil, "Filter repos by tags, any-match (comma-separated or repeated flag)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	short, _ := cmd.Flags().GetBool("short")
	jsonOut, _ := cmd.Flags().GetBool("json")
	out := cmd.OutOrStdout()

	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	// When no workspace name is given, check if we're in a standalone repo.
	// If the user explicitly provides a name, skip detection and go straight
	// to workspace resolution.
	if nameArg == "" {
		cwd, err := currentDir()
		if err != nil {
			return err
		}
		det, err := workspace.Detect(cwd, execRunner())
		if err != nil {
			return fmt.Errorf("detecting workspace: %w", err)
		}

		if det.IsStandaloneRepo {
			name := filepath.Base(det.StandaloneRepoRoot)
			entry := workspace.GatherSingleRepoStatus(det.StandaloneRepoRoot, name, "", execRunner())
			if jsonOut {
				// Standalone-repo mode: same document shape with workspace ""
				// and a single entry (group "", tags []) — see output.Status.
				return printJSON(cmd, output.NewStatus("", []workspace.RepoStatusEntry{entry}))
			}
			if short {
				fmt.Fprintln(out, tui.ShortRepoLine(entry))
			} else {
				fmt.Fprint(out, tui.RenderRepoTable([]workspace.RepoStatusEntry{entry}))
			}
			return nil
		}
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

	// Apply filter flags to restrict which repos appear in the table.
	filterOpts, err := filterOptsFromFlags(cmd, nil)
	if err != nil {
		return err
	}
	filteredRepos := workspace.ApplyRepoFilter(wsCfg.Repos, filterOpts)
	statusCfg := wsCfg
	statusCfg.Repos = filteredRepos

	if len(statusCfg.Repos) == 0 {
		if jsonOut {
			// DECISION: a filter matching nothing still emits the full
			// document shape with "repos": [] and exit 0 — the hint text is
			// for humans; JSON consumers see the filter result directly.
			return printJSON(cmd, output.NewStatus(name, nil))
		}
		fmt.Fprintln(out, "no repos matched the filter")
		return nil
	}

	statuses, err := workspace.GatherStatus(
		statusCfg,
		wsDir,
		execRunner(),
		globalCfg.Parallel.Enabled,
		globalCfg.Parallel.BatchSize,
	)
	if err != nil {
		return fmt.Errorf("gathering status: %w", err)
	}

	if jsonOut {
		// DECISION: tags travel in the JSON document only — the human table
		// and --short line keep their existing columns. Adding tags would
		// widen the table for a field the table user rarely needs and would
		// shift --short's tab-separated columns under existing scripts.
		return printJSON(cmd, output.NewStatus(name, statuses))
	}

	if short {
		for _, s := range statuses {
			fmt.Fprintln(out, tui.ShortRepoLine(s))
		}
		return nil
	}

	// Surface an active show filter so a filtered VS Code view is never
	// silently in effect (ergo-vscode-spec.md §3.2). Table format only:
	// --short stays strictly one-line-per-repo for scripts.
	wsFilePath := filepath.Join(wsDir, name+".code-workspace")
	if f := preservedFilter(wsFilePath); f != nil {
		visible := len(workspace.ApplyRepoFilter(wsCfg.Repos, showFilterOptions(f)))
		fmt.Fprintln(out, filterNote(f, visible, len(wsCfg.Repos)))
	}

	fmt.Fprint(out, tui.RenderRepoTable(statuses))
	return nil
}
