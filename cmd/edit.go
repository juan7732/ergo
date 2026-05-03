package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
)

var (
	editGlobal bool
)

var editCmd = &cobra.Command{
	Use:   "edit [workspace-name]",
	Short: "Open the workspace TOML in VS Code",
	Long: `Open the workspace TOML configuration file in VS Code for editing.

Changes take effect the next time 'ergo sync' or 'ergo open' is run.

Use --global to open the global ergo config (~/.ergo/config.toml) instead of
a workspace TOML.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEdit,
}

func init() {
	editCmd.Flags().BoolVarP(&editGlobal, "global", "g", false, "edit the global ergo config (~/.ergo/config.toml)")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	var targetPath string

	if editGlobal {
		if len(args) > 0 {
			return fmt.Errorf("--global does not take a workspace name")
		}
		home, err := config.ErgoHome()
		if err != nil {
			return fmt.Errorf("getting ergo home: %w", err)
		}
		// Ensure the global config exists on disk before opening (LoadGlobal
		// writes the defaults file when it's missing).
		if _, err := config.LoadGlobal(); err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}
		targetPath = filepath.Join(home, "config.toml")
	} else {
		nameArg := ""
		if len(args) > 0 {
			nameArg = args[0]
		}

		name, err := resolveWorkspaceName(cmd, nameArg)
		if err != nil {
			return err
		}

		home, err := config.ErgoHome()
		if err != nil {
			return fmt.Errorf("getting ergo home: %w", err)
		}

		targetPath = filepath.Join(home, "workspaces", name+".toml")
	}

	codePath, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("'code' not found on PATH: install the VS Code CLI and retry (https://code.visualstudio.com/docs/setup/mac)")
	}

	c := exec.Command(codePath, targetPath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("launching VS Code: %w", err)
	}
	return nil
}
