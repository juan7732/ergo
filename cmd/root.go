package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var version string

var rootCmd = &cobra.Command{
	Use:   "ergo",
	Short: "Multi-repo VS Code workspace manager",
	Long:  `ergo manages multi-repo development workspaces. It clones repos, organizes them into a working directory, generates VS Code workspace files, and provides commands to operate across all repos simultaneously.`,
	// No Run: ergo with no subcommand prints help
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags.
// Called by main.main() with the version string embedded at build time.
//
// rootCmd.Execute already prints "Error: <msg>" to stderr for any RunE
// failure, so only the exit code is handled here. Wrapping it in
// cobra.CheckErr printed every error twice.
func Execute(v string) {
	version = v
	rootCmd.Version = v
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags (workspace-name flag added per-command as needed)
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")

	rootCmd.AddCommand(updateCmd)
}
