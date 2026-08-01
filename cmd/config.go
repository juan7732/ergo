package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config [workspace-name]",
	Short: "Print a workspace's TOML configuration",
	Long: `Print a workspace's configuration to stdout. Read-only — the counterpart
to 'ergo edit', which opens the TOML for modification.

Without flags, prints the workspace TOML verbatim. With --json, prints the
configuration normalized to JSON: repo names are resolved to their effective
form (explicit name or derived from the URL), so consumers never reimplement
name derivation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfig,
}

func init() {
	configCmd.Flags().Bool("json", false, "Print the configuration as a normalized JSON document")
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	if jsonOut {
		wsCfg, err := config.LoadWorkspace(name)
		if err != nil {
			return fmt.Errorf("loading workspace config: %w", err)
		}
		return printJSON(cmd, output.NewConfig(name, wsCfg))
	}

	// DECISION: the human form prints the TOML file verbatim (cat semantics)
	// rather than a re-rendered view — the file is the source of truth and
	// any pretty-printing would just be a lossier copy of it.
	home, err := config.ErgoHome()
	if err != nil {
		return fmt.Errorf("getting ergo home: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, "workspaces", name+".toml"))
	if err != nil {
		return fmt.Errorf("reading workspace config: %w", err)
	}
	_, err = cmd.OutOrStdout().Write(raw)
	return err
}
