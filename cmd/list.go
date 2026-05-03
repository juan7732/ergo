package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
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
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	names, err := config.ListWorkspaceNames()
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}

	if len(names) == 0 {
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

	type listRow struct {
		name   string
		repos  int
		status string
	}

	rows := make([]listRow, 0, len(names))
	for _, name := range names {
		wsCfg, err := config.LoadWorkspace(name)
		if err != nil {
			// Skip unreadable configs with a warning rather than aborting.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s: %v\n", name, err)
			continue
		}

		status := "not synced"
		wsDir := filepath.Join(wsRoot, name)
		if info, statErr := os.Stat(wsDir); statErr == nil && info.IsDir() {
			status = "synced"
		}

		rows = append(rows, listRow{
			name:   name,
			repos:  len(wsCfg.Repos),
			status: status,
		})
	}

	// Render as a table.
	out := cmd.OutOrStdout()
	wName, wRepos, wStatus := len("Workspace"), len("Repos"), len("Status")
	for _, r := range rows {
		if n := len(r.name); n > wName {
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
		statusStr := r.status
		if r.status == "synced" {
			statusStr = tui.StyleSuccess.Render(r.status)
		} else {
			statusStr = tui.StyleSubtle.Render(r.status)
		}
		fmt.Fprintf(out, "│ %-*s │ %-*d │ %-*s │\n",
			wName, r.name,
			wRepos, r.repos,
			wStatus, statusStr,
		)
	}
	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("└", "┴", "┘")))
	return nil
}
