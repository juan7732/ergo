package github

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ergoRepo is the canonical GitHub repository for self-update.
// Hardcoded by design — self-update has exactly one source.
const ergoRepo = "juan7732/ergo"

// Runner executes a shell command in a given directory and returns trimmed stdout.
// The thin interface exists to enable test fakes without spawning real gh processes.
type Runner interface {
	Run(dir, name string, args ...string) (string, error)
}

// ExecRunner is the default Runner that shells out via os/exec.
type ExecRunner struct{}

// Run executes name with args in dir and returns trimmed stdout.
func (ExecRunner) Run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CheckPath returns an error if gh is not found on PATH.
func CheckPath() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh is required")
	}
	return nil
}

// LatestRelease returns the tag name of the most recent release on ergoRepo
// (e.g. "v0.2.0"). Returns an error if gh is unavailable or the release
// list is empty.
func LatestRelease(r Runner) (string, error) {
	out, err := r.Run("", "gh", "release", "list",
		"--repo", ergoRepo,
		"--limit", "1",
		"--json", "tagName",
		"--jq", ".[0].tagName",
	)
	if err != nil {
		return "", fmt.Errorf("listing releases for %s: %w", ergoRepo, err)
	}
	tag := strings.TrimSpace(out)
	if tag == "" {
		return "", fmt.Errorf("no releases found for %s", ergoRepo)
	}
	return tag, nil
}

// ChecksumName is the asset name goreleaser publishes the SHA-256 manifest
// under (configured via .goreleaser.yaml's checksum.name_template). Hardcoded
// for the same reason ergoRepo is: self-update has exactly one source layout.
const ChecksumName = "checksums.txt"

// ChecksumFor parses a goreleaser-style checksums file (lines of
// "<sha256>  <filename>") and returns the hex digest recorded for assetName.
func ChecksumFor(checksumsPath, assetName string) (string, error) {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("reading checksums file: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// The filename is the last field; sha256sum -b prefixes it with '*'.
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == assetName {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", assetName)
}

// FileSHA256 returns the lowercase hex SHA-256 digest of the file at path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s for checksum: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DownloadRelease downloads the asset matching assetPattern from the release
// tagged tag into destDir.
func DownloadRelease(r Runner, tag, assetPattern, destDir string) error {
	_, err := r.Run("", "gh", "release", "download", tag,
		"--repo", ergoRepo,
		"--pattern", assetPattern,
		"--dir", destDir,
	)
	if err != nil {
		return fmt.Errorf("downloading release %s (%s): %w", tag, assetPattern, err)
	}
	return nil
}
