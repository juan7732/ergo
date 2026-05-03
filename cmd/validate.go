package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/git"
	"github.com/juan7732/ergo/internal/tui"
	"github.com/juan7732/ergo/internal/workspace"
)

var validateCmd = &cobra.Command{
	Use:   "validate [workspace-name]",
	Short: "Validate a workspace TOML configuration",
	Long: `Validate the TOML configuration for a workspace.

Checks for required fields, name collisions, and other spec constraints.
Exits with a non-zero status code if any issues are found.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().Bool("all", false, "Validate all workspaces")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all {
		return validateAll(cmd)
	}

	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	return validateOne(cmd, name)
}

// validateAll validates every workspace and prints all issues.
func validateAll(cmd *cobra.Command) error {
	names, err := config.ListWorkspaceNames()
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}
	if len(names) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no workspaces defined")
		return nil
	}

	hadErrors := false
	for _, name := range names {
		if err := validateOne(cmd, name); err != nil {
			hadErrors = true
		}
	}
	if hadErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// validateOne loads and validates a single workspace, printing issues.
func validateOne(cmd *cobra.Command, name string) error {
	out := cmd.OutOrStdout()

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		fmt.Fprintf(out, "%s  %s\n",
			tui.StyleError.Render("✗ "+name),
			tui.StyleSubtle.Render(err.Error()),
		)
		return err
	}

	valErr := config.Validate(wsCfg)
	if valErr == nil {
		fmt.Fprintf(out, "%s\n", tui.StyleSuccess.Render("✓ "+name))
		return nil
	}

	var ve config.ValidationErrors
	if !errors.As(valErr, &ve) {
		fmt.Fprintf(out, "%s  %s\n",
			tui.StyleError.Render("✗ "+name),
			tui.StyleSubtle.Render(valErr.Error()),
		)
		return valErr
	}

	fmt.Fprintf(out, "%s  %s\n",
		tui.StyleError.Render("✗ "+name),
		tui.StyleSubtle.Render(fmt.Sprintf("%d issue(s)", len(ve))),
	)
	for _, e := range ve {
		if e.Field != "" {
			fmt.Fprintf(out, "  %s %s: %s\n",
				tui.StyleSubtle.Render("•"),
				tui.StyleLabel.Render(e.Field),
				e.Message,
			)
		} else {
			fmt.Fprintf(out, "  %s %s\n",
				tui.StyleSubtle.Render("•"),
				e.Message,
			)
		}
	}
	return valErr
}

// resolveWorkspaceName resolves a workspace name using the standard resolution
// order, launching the TUI selector when ambiguous.
func resolveWorkspaceName(cmd *cobra.Command, nameArg string) (string, error) {
	cwd, err := currentDir()
	if err != nil {
		return "", err
	}

	result, err := workspace.Resolve(nameArg, cwd, git.ExecRunner{})
	if err != nil {
		return "", err
	}

	if result.Name != "" {
		return result.Name, nil
	}

	// Ambiguous: show TUI selector.
	sel := tui.NewWorkspaceSelect(result.Candidates, nameArg)
	finalModel, err := tui.RunInline(sel)
	if err != nil {
		return "", fmt.Errorf("running workspace selector: %w", err)
	}

	name, ok := finalModel.(tui.WorkspaceSelect).Result()
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
		return "", fmt.Errorf("no workspace selected")
	}
	return name, nil
}
