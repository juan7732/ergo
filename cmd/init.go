package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/tui"
)

var initCmd = &cobra.Command{
	Use:   "init [workspace-name]",
	Short: "Create a new workspace definition",
	Long: `Create a new workspace definition via a guided TUI flow.

Writes ~/.ergo/workspaces/<workspace-name>.toml with the repos and folders
you configure. Does not create directories or clone anything — use 'ergo open'
to materialize the workspace.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	prefillName := ""
	if len(args) > 0 {
		prefillName = args[0]
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wizard := tui.NewInitWizard(prefillName, globalCfg.Defaults.DefaultBranch)
	finalModel, err := tui.RunInline(wizard)
	if err != nil {
		return fmt.Errorf("running init wizard: %w", err)
	}

	result, confirmed := finalModel.(tui.InitWizard).Result()
	if !confirmed {
		fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
		return nil
	}

	wsCfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{
			Name: result.Name,
		},
		Repos:   result.Repos,
		Folders: result.Folders,
	}

	if err := config.WriteWorkspace(result.Name, wsCfg); err != nil {
		return fmt.Errorf("writing workspace config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "workspace %q created\n", result.Name)
	return nil
}
