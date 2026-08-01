package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/output"
	"github.com/juan7732/ergo/internal/tui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured workspaces",
	Long:  `List all workspaces defined in ~/.ergo/workspaces/ with their sync status.`,
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().Bool("json", false, "Print a machine-readable JSON document instead of the table")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	names, err := config.ListWorkspaceNames()
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}

	if len(names) == 0 {
		if jsonOut {
			// Empty state is {"workspaces": []}, exit 0 — the hint text is
			// for humans only.
			return printJSON(cmd, output.NewList(nil))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "no workspaces defined — run 'ergo init' to create one")
		return nil
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsRoot, err := config.ExpandTilde(globalCfg.Defaults.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("expanding workspace root: %w", err)
	}

	rows := make([]output.ListWorkspace, 0, len(names))
	for _, name := range names {
		wsCfg, err := config.LoadWorkspace(name)
		if err != nil {
			// Skip unreadable configs with a warning rather than aborting.
			// The warning goes to stderr in both output modes, so --json
			// consumers still get a clean document on stdout.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s: %v\n", name, err)
			continue
		}

		synced := false
		wsDir := filepath.Join(wsRoot, name)
		if info, statErr := os.Stat(wsDir); statErr == nil && info.IsDir() {
			synced = true
		}

		rows = append(rows, output.ListWorkspace{
			Name:   name,
			Repos:  len(wsCfg.Repos),
			Synced: synced,
		})
	}

	if jsonOut {
		return printJSON(cmd, output.NewList(rows))
	}

	// Render as a table.
	out := cmd.OutOrStdout()
	wName, wRepos, wStatus := len("Workspace"), len("Repos"), len("Status")
	for _, r := range rows {
		if n := len(r.Name); n > wName {
			wName = n
		}
	}

	border := func(left, cross, right string) string {
		return left + strings.Repeat("─", wName+2) + cross + strings.Repeat("─", wRepos+2) + cross + strings.Repeat("─", wStatus+2) + right
	}

	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("┌", "┬", "┐")))
	fmt.Fprintf(out, "│ %-*s │ %-*s │ %-*s │\n",
		wName, tui.StyleTableHeader.Render("Workspace"),
		wRepos, tui.StyleTableHeader.Render("Repos"),
		wStatus, tui.StyleTableHeader.Render("Status"),
	)
	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("├", "┼", "┤")))
	// REVIEW: %-*s counts bytes, but lipgloss-styled strings embed invisible ANSI
	// escape codes that inflate byte length without adding visible width. The Status
	// column will visually misalign when ANSI is present. Fix in a follow-up by
	// tracking visible width separately (e.g. via lipgloss.Width) and padding manually.
	for _, r := range rows {
		var statusStr string
		if r.Synced {
			statusStr = tui.StyleSuccess.Render("synced")
		} else {
			statusStr = tui.StyleSubtle.Render("not synced")
		}
		fmt.Fprintf(out, "│ %-*s │ %-*d │ %-*s │\n",
			wName, r.Name,
			wRepos, r.Repos,
			wStatus, statusStr,
		)
	}
	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("└", "┴", "┘")))
	return nil
}
