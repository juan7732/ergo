package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/output"
	"github.com/juan7732/ergo/internal/tui"
	"github.com/juan7732/ergo/internal/workspace"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Find repos, folders, and workspaces by name across all workspaces",
	Long: `Search every workspace TOML under ~/.ergo/workspaces/ for repos, folders,
and workspaces whose name contains <query> (case-insensitive substring; repo
URLs are matched too). Each hit reports the workspace it belongs to, its kind,
and whether it is actually on disk: cloned for repos, created for folders,
synced for workspaces.

Answers "do I already have this repo somewhere?" without grepping TOML files
by hand. Reads only config files and directory entries: no git, no network.`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().Bool("json", false, "Print a machine-readable JSON document instead of the table")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	query := args[0]

	// DECISION: an empty or blank query is rejected rather than treated as
	// "match everything". Substring matching would make "" match every entry,
	// which is almost always an unset shell variable, not a request to dump
	// all workspaces (that is what ergo list and ergo config are for).
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("search query must not be empty")
	}

	names, err := config.ListWorkspaceNames()
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsRoot, err := config.ExpandTilde(globalCfg.Defaults.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("expanding workspace root: %w", err)
	}

	workspaces := make([]workspace.NamedConfig, 0, len(names))
	for _, name := range names {
		wsCfg, err := config.LoadWorkspace(name)
		if err != nil {
			// Same policy as ergo list: one broken workspace must not hide
			// hits in healthy ones. The warning goes to stderr in both
			// output modes, so --json consumers still get a clean document
			// on stdout.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s: %v\n", name, err)
			continue
		}
		workspaces = append(workspaces, workspace.NamedConfig{Name: name, Config: wsCfg})
	}

	hits := workspace.Search(query, workspaces, wsRoot)

	if jsonOut {
		return printJSON(cmd, output.NewSearch(query, hits))
	}

	out := cmd.OutOrStdout()

	// DECISION: no hits is a successful query and exits 0, consistent with
	// `ergo list --json` printing {"workspaces": []} and with status filters
	// that match nothing. Scripts test emptiness with
	// `jq -e '.results | length > 0'`.
	// REVIEW: grep-style exit 1 on no match was the considered alternative;
	// it composes better with `&&` chains but breaks the "exit codes are
	// unchanged between output modes" invariant unless --json follows suit.
	if len(hits) == 0 {
		fmt.Fprintf(out, "no matches for %q\n", query)
		return nil
	}

	printSearchTable(out, hits)
	return nil
}

// printSearchTable renders hits as a bordered table in ergo list's style.
//
// DECISION: the table shows workspace, kind, name, group, tags, and state
// only. URL and path are JSON-only: they are long, and for a human the
// workspace plus name already answers "where is it?", while the path exists
// precisely so scripts never re-derive the workspace root join.
func printSearchTable(out io.Writer, hits []workspace.Hit) {
	headers := []string{"Workspace", "Kind", "Name", "Group", "Tags", "State"}
	rows := make([][]string, 0, len(hits))
	for _, h := range hits {
		rows = append(rows, []string{
			h.Workspace,
			string(h.Kind),
			h.Name,
			h.Group,
			strings.Join(h.Tags, ", "),
			searchStateLabel(h),
		})
	}

	// Column widths are computed from the visible width of each cell
	// (lipgloss.Width ignores ANSI escapes), so styling the state cell and
	// the headers cannot misalign the table. This is the fix list.go's
	// REVIEW comment asks for; list.go keeps its own renderer untouched.
	widths := make([]int, len(headers))
	for i, hdr := range headers {
		widths[i] = lipgloss.Width(hdr)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	border := func(left, cross, right string) string {
		parts := make([]string, len(widths))
		for i, w := range widths {
			parts[i] = strings.Repeat("─", w+2)
		}
		return left + strings.Join(parts, cross) + right
	}
	line := func(cells []string) string {
		padded := make([]string, len(cells))
		for i, c := range cells {
			padded[i] = padCell(c, widths[i])
		}
		return "│ " + strings.Join(padded, " │ ") + " │"
	}

	styledHeaders := make([]string, len(headers))
	for i, hdr := range headers {
		styledHeaders[i] = tui.StyleTableHeader.Render(hdr)
	}

	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("┌", "┬", "┐")))
	fmt.Fprintln(out, line(styledHeaders))
	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("├", "┼", "┤")))
	for _, row := range rows {
		fmt.Fprintln(out, line(row))
	}
	fmt.Fprintln(out, tui.StyleTableBorder.Render(border("└", "┴", "┘")))
}

// searchStateLabel renders the kind-specific on-disk state, styled so
// present and absent are distinguishable at a glance.
func searchStateLabel(h workspace.Hit) string {
	var present, absent string
	switch h.Kind {
	case workspace.HitKindRepo:
		present, absent = "cloned", "uncloned"
	case workspace.HitKindFolder:
		present, absent = "created", "not created"
	default:
		present, absent = "synced", "not synced"
	}
	if h.Exists {
		return tui.StyleSuccess.Render(present)
	}
	return tui.StyleSubtle.Render(absent)
}

// padCell right-pads s with spaces to the given visible width, measuring
// with lipgloss.Width so ANSI escapes in styled cells do not count.
func padCell(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
