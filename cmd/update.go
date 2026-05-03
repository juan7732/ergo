package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/juan7732/ergo/internal/github"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for a new version and update the binary",
	Long:  `Checks GitHub releases for a newer version of ergo and replaces the running binary atomically.`,
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	runner := github.ExecRunner{}

	if err := github.CheckPath(); err != nil {
		return err
	}

	latest, err := github.LatestRelease(runner)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	// Normalize by stripping leading 'v' for comparison.
	latestVer := strings.TrimPrefix(latest, "v")
	currentVer := strings.TrimPrefix(version, "v")

	// "dev" builds always attempt an update.
	if currentVer == latestVer && currentVer != "dev" {
		fmt.Fprintf(cmd.OutOrStdout(), "ergo is already up to date (%s)\n", latest)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updating ergo %s → %s\n", version, latest)

	// Resolve the real path of the running binary.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks for executable: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	const assetName = "ergo-darwin-arm64"
	downloadedPath := filepath.Join(exeDir, assetName)

	// Clean up downloaded asset on any failure after this point.
	var downloaded bool
	defer func() {
		if !downloaded {
			_ = os.Remove(downloadedPath)
		}
	}()

	if err := github.DownloadRelease(runner, latest, assetName, exeDir); err != nil {
		return err
	}
	downloaded = true

	if err := os.Chmod(downloadedPath, 0o755); err != nil {
		return fmt.Errorf("setting permissions on downloaded binary: %w", err)
	}

	// os.Rename is atomic on the same filesystem, replacing the running binary.
	if err := os.Rename(downloadedPath, exePath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	downloaded = false // rename consumed the file; skip deferred remove

	fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s\n", latest)
	return nil
}
