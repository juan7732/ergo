package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/tui"
	"github.com/juan7732/ergo/internal/vscode"
	"github.com/juan7732/ergo/internal/workspace"
)

var showCmd = &cobra.Command{
	Use:   "show [group | all]",
	Short: "Filter the workspace view to a group or tags",
	Long: `Filter the VS Code workspace view by regenerating .code-workspace to include
only repos matching the filter. The filter is recorded in the ergo metadata
object so it persists between runs.

  ergo show ml           # filter to the "ml" group
  ergo show --tag=go     # filter to repos tagged "go"
  ergo show all          # clear the filter, restore full view
  ergo show              # interactive group/tag selector

Modifies .code-workspace only — never the TOML or filesystem.
The workspace must be materialized on disk (run 'ergo open' first).

ergo show operates on the workspace detected from the current working directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runShow,
}

func init() {
	showCmd.Flags().StringSlice("tag", nil, "Filter repos by tag (can be repeated or comma-separated)")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	tags, _ := cmd.Flags().GetStringSlice("tag")
	out := cmd.OutOrStdout()

	// show always operates on the workspace detected from CWD — the positional
	// arg is reserved for the group name, not a workspace name.
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	det, err := workspace.Detect(cwd, execRunner())
	if err != nil {
		return fmt.Errorf("detecting workspace: %w", err)
	}
	if det.WorkspaceName == "" {
		return fmt.Errorf("not inside an ergo workspace; cd into the workspace directory or run 'ergo open' first")
	}

	name := det.WorkspaceName

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
	if _, statErr := os.Stat(wsFilePath); os.IsNotExist(statErr) {
		return fmt.Errorf("workspace not materialized; run 'ergo open %s' first", name)
	}

	// Determine the filter from args, flags, or TUI.
	var filterGroup string
	var filterTags []string
	clearFilter := false

	if len(args) > 0 {
		if args[0] == "all" {
			clearFilter = true
		} else {
			filterGroup = args[0]
		}
	} else if len(tags) > 0 {
		filterTags = tags
	} else {
		// No positional arg and no --tag flag: launch group/tag selector TUI.
		items := buildGroupSelectItems(wsCfg)
		if len(items) == 0 {
			return fmt.Errorf("no groups or tags defined in workspace")
		}
		sel := tui.NewGroupSelect(items)
		finalModel, tuiErr := tui.RunInline(sel)
		if tuiErr != nil {
			return fmt.Errorf("running group selector: %w", tuiErr)
		}
		result, ok := finalModel.(tui.GroupSelect).Result()
		if !ok {
			fmt.Fprintln(out, "cancelled")
			return nil
		}
		filterGroup = result.Group
		filterTags = result.Tags
	}

	if clearFilter {
		b, genErr := vscode.Generate(wsCfg, nil)
		if genErr != nil {
			return fmt.Errorf("generating .code-workspace: %w", genErr)
		}
		written, writeErr := vscode.WriteIfChanged(wsFilePath, b)
		if writeErr != nil {
			return fmt.Errorf("writing .code-workspace: %w", writeErr)
		}
		if written {
			fmt.Fprintln(out, "filter cleared — workspace restored to full view")
		} else {
			fmt.Fprintln(out, "no filter was active")
		}
		return nil
	}

	// Apply filter to repos; always include all folders in the filtered view.
	filterOpts := workspace.FilterOptions{
		Group: filterGroup,
		Tags:  filterTags,
	}
	filteredRepos := workspace.ApplyRepoFilter(wsCfg.Repos, filterOpts)
	if len(filteredRepos) == 0 {
		return fmt.Errorf("no repos matched the filter")
	}

	filteredCfg := wsCfg
	filteredCfg.Repos = filteredRepos

	vsFilter := &vscode.Filter{
		Group: filterGroup,
		Tags:  filterTags,
	}
	b, genErr := vscode.Generate(filteredCfg, vsFilter)
	if genErr != nil {
		return fmt.Errorf("generating .code-workspace: %w", genErr)
	}
	written, writeErr := vscode.WriteIfChanged(wsFilePath, b)
	if writeErr != nil {
		return fmt.Errorf("writing .code-workspace: %w", writeErr)
	}

	if filterGroup != "" {
		if written {
			fmt.Fprintf(out, "filter set to group %q (%d repo(s))\n", filterGroup, len(filteredRepos))
		} else {
			fmt.Fprintf(out, "filter already set to group %q\n", filterGroup)
		}
	} else {
		if written {
			fmt.Fprintf(out, "filter set to tags %v (%d repo(s))\n", filterTags, len(filteredRepos))
		} else {
			fmt.Fprintf(out, "filter already set to tags %v\n", filterTags)
		}
	}
	return nil
}

// buildGroupSelectItems collects unique groups (first) and unique tags (after)
// from all repos, preserving first-seen order.
func buildGroupSelectItems(cfg config.WorkspaceConfig) []tui.GroupSelectItem {
	groupSeen := make(map[string]bool)
	tagSeen := make(map[string]bool)

	var items []tui.GroupSelectItem
	for _, repo := range cfg.Repos {
		if repo.Group != "" && !groupSeen[repo.Group] {
			groupSeen[repo.Group] = true
			items = append(items, tui.GroupSelectItem{Name: repo.Group, IsTag: false})
		}
	}
	for _, repo := range cfg.Repos {
		for _, tag := range repo.Tags {
			if !tagSeen[tag] {
				tagSeen[tag] = true
				items = append(items, tui.GroupSelectItem{Name: tag, IsTag: true})
			}
		}
	}
	return items
}
