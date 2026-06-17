//go:build integration

package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stretchr/testify/require"
)

// InstallCodeStub installs a `code` shim in the harness PATH dir that records
// each invocation (one line: <arg> per line, blank line separating invocations)
// to <Home>/.code-invocations.log.
//
// Returns the path to the log file. Use CodeInvocations() to read it.
func (h *Harness) InstallCodeStub() string {
	h.t.Helper()

	logPath := filepath.Join(h.Home, ".code-invocations.log")

	script := fmt.Sprintf(`#!/usr/bin/env bash
{
  for arg in "$@"; do
    printf '%%s\n' "$arg"
  done
  printf '\n'
} >> %q
exit 0
`, logPath)

	require.NoError(h.t, os.WriteFile(filepath.Join(h.PathDir, "code"), []byte(script), 0o755))
	return logPath
}

// CodeInvocations returns each `code` invocation as a slice of string slices
// (one inner slice per invocation, holding its argv).
func (h *Harness) CodeInvocations() [][]string {
	h.t.Helper()
	logPath := filepath.Join(h.Home, ".code-invocations.log")
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(h.t, err)

	var invocations [][]string
	for _, block := range strings.Split(strings.TrimRight(string(b), "\n"), "\n\n") {
		if block == "" {
			continue
		}
		invocations = append(invocations, strings.Split(block, "\n"))
	}
	return invocations
}

// GhStubOptions configures the canned responses served by the gh stub.
type GhStubOptions struct {
	// LatestTag is the tag returned for `gh release list ... --jq .[0].tagName`.
	LatestTag string
	// AssetSource is a path on disk whose contents will be served when
	// `gh release download <tag> --pattern <name> --dir <dir>` is invoked.
	// The asset is copied to <dir>/<pattern>. If empty, a 1-byte placeholder is used.
	AssetSource string
	// CorruptChecksum, when true, makes the served checksums.txt list a wrong
	// hash for the asset so the update path's verification must fail.
	CorruptChecksum bool
}

// runtimeAssetName is the release-asset name `ergo update` derives from the
// running binary's platform (ergo-<goos>-<goarch>). The integration binary and
// the test runner share a platform, so this matches what update will request.
func runtimeAssetName() string {
	return fmt.Sprintf("ergo-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// InstallGhStub installs a `gh` shim in the harness PATH dir that serves canned
// responses based on opts. Supports:
//
//	gh release list --repo <r> --limit 1 --json tagName --jq .[0].tagName
//	gh release download <tag> --repo <r> --pattern <name> --dir <dir>
//
// A download of the asset copies AssetSource verbatim. A download of
// "checksums.txt" synthesizes a goreleaser-style checksums file whose entry for
// the runtime asset is the real SHA-256 of AssetSource (or a wrong hash when
// CorruptChecksum is set).
func (h *Harness) InstallGhStub(opts GhStubOptions) {
	h.t.Helper()

	source := opts.AssetSource
	if source == "" {
		// Write a placeholder file in the harness dir.
		source = filepath.Join(h.Home, ".gh-stub-asset")
		require.NoError(h.t, os.WriteFile(source, []byte("ergo-stub\n"), 0o755))
	}

	checksumExpr := `$(sha256sum "$src" | cut -d" " -f1)`
	if opts.CorruptChecksum {
		checksumExpr = `0000000000000000000000000000000000000000000000000000000000000000`
	}

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

src=%q
asset=%q

if [[ "${1:-}" == "release" && "${2:-}" == "list" ]]; then
  printf '%%s\n' %q
  exit 0
fi

if [[ "${1:-}" == "release" && "${2:-}" == "download" ]]; then
  tag="${3:-}"
  pattern=""
  dir=""
  shift 3
  while (( $# )); do
    case "$1" in
      --pattern) pattern="$2"; shift 2 ;;
      --dir)     dir="$2";     shift 2 ;;
      *)                       shift   ;;
    esac
  done
  : "$tag"
  if [[ -z "$pattern" || -z "$dir" ]]; then
    echo "stub: missing --pattern or --dir" >&2
    exit 2
  fi
  mkdir -p "$dir"
  if [[ "$pattern" == "checksums.txt" ]]; then
    sum=%s
    printf '%%s  %%s\n' "$sum" "$asset" > "$dir/$pattern"
  else
    cp "$src" "$dir/$pattern"
  fi
  exit 0
fi

echo "gh stub: unsupported invocation: $*" >&2
exit 2
`, source, runtimeAssetName(), opts.LatestTag, checksumExpr)

	require.NoError(h.t, os.WriteFile(filepath.Join(h.PathDir, "gh"), []byte(script), 0o755))
}
