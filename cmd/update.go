package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// homebrewPrefixes returns the candidate Homebrew install prefixes to test the
// running binary against, $HOMEBREW_PREFIX first when set.
func homebrewPrefixes() []string {
	prefixes := []string{"/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"}
	if p := os.Getenv("HOMEBREW_PREFIX"); p != "" {
		prefixes = append([]string{p}, prefixes...)
	}
	return prefixes
}

// homebrewPrefixFor reports the Homebrew prefix that exePath lives under, if any.
func homebrewPrefixFor(exePath string) (string, bool) {
	exePath = filepath.Clean(exePath)
	for _, prefix := range homebrewPrefixes() {
		prefix = filepath.Clean(prefix)
		if exePath == prefix || strings.HasPrefix(exePath, prefix+string(filepath.Separator)) {
			return prefix, true
		}
	}
	return "", false
}

func runUpdate(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	// Resolve the real path of the running binary up front; it drives both the
	// managed-install check and the eventual atomic swap target.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks for executable: %w", err)
	}

	// DECISION: A Homebrew-managed install must not self-replace its binary —
	// doing so fights the package manager (brew owns the file under its Cellar,
	// and a hand-swapped binary is clobbered on the next `brew upgrade`). The
	// spec is silent on detection mechanics, so we follow the project task's
	// rule: treat the install as managed when the resolved binary lives under a
	// Homebrew prefix ($HOMEBREW_PREFIX, else the well-known defaults). Only the
	// standalone release-download install path self-updates.
	if prefix, managed := homebrewPrefixFor(exePath); managed {
		fmt.Fprintf(out, "ergo was installed via Homebrew (%s) — run 'brew upgrade ergo' to update\n", prefix)
		return nil
	}

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
		fmt.Fprintf(out, "ergo is already up to date (%s)\n", latest)
		return nil
	}

	fmt.Fprintf(out, "Updating ergo %s → %s\n", version, latest)

	exeDir := filepath.Dir(exePath)
	// Release assets are named per-platform (ergo-<goos>-<goarch>), matching the
	// goreleaser build matrix, so the right binary is fetched on every platform.
	assetName := fmt.Sprintf("ergo-%s-%s", runtime.GOOS, runtime.GOARCH)
	downloadedPath := filepath.Join(exeDir, assetName)
	checksumsPath := filepath.Join(exeDir, github.ChecksumName)

	// Clean up downloaded artifacts on any failure after this point.
	var downloaded bool
	defer func() {
		_ = os.Remove(checksumsPath)
		if !downloaded {
			_ = os.Remove(downloadedPath)
		}
	}()

	if err := github.DownloadRelease(runner, latest, assetName, exeDir); err != nil {
		return err
	}
	downloaded = true

	// Verify the asset against the release checksums before trusting it.
	if err := github.DownloadRelease(runner, latest, github.ChecksumName, exeDir); err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	expected, err := github.ChecksumFor(checksumsPath, assetName)
	if err != nil {
		return err
	}
	actual, err := github.FileSHA256(downloadedPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expected, actual)
	}

	if err := os.Chmod(downloadedPath, 0o755); err != nil {
		return fmt.Errorf("setting permissions on downloaded binary: %w", err)
	}

	// os.Rename is atomic on the same filesystem, replacing the running binary.
	if err := os.Rename(downloadedPath, exePath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	downloaded = false // rename consumed the file; skip deferred remove

	fmt.Fprintf(out, "Updated to %s\n", latest)
	return nil
}
