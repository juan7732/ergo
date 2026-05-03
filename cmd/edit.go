package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
)

var editCmd = &cobra.Command{
	Use:   "edit [workspace-name]",
	Short: "Open the workspace TOML in VS Code",
	Long: `Open the workspace TOML configuration file in VS Code for editing.

Changes take effect the next time 'ergo sync' or 'ergo open' is run.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
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

	tomlPath := filepath.Join(home, "workspaces", name+".toml")

	codePath, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("'code' not found on PATH: install the VS Code CLI and retry (https://code.visualstudio.com/docs/setup/mac)")
	}

	c := exec.Command(codePath, tomlPath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("launching VS Code: %w", err)
	}
	return nil
}
