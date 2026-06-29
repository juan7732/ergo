//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// stageBinary copies the integration-built binary to dst and points the harness
// at it, so `ergo update`'s os.Executable()+atomic-rename targets a throwaway
// path rather than the shared integration binary.
func stageBinary(t *testing.T, h *harness.Harness, dst string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	src, err := os.ReadFile(harness.ErgoBinary())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, src, 0o755))
	h.Binary = dst
}

// TestUpdate_AtomicReplace stubs `gh` to serve a newer release and asserts that
// the `update` command verifies the checksum and replaces the binary at the
// path it was launched from.
//
// We don't run the post-swap binary (the asset is a placeholder, not a real ergo
// build); we only verify the swap happens.
func TestUpdate_AtomicReplace(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	stagingBin := filepath.Join(h.Home, "bin", "ergo")
	stageBinary(t, h, stagingBin)

	// Stub gh to advertise a newer release and serve a placeholder asset plus a
	// matching checksums file. Use a tag that won't match the embedded
	// "integration" version so update proceeds.
	h.InstallGhStub(harness.GhStubOptions{
		LatestTag: "v999.0.0",
	})

	origInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)
	origSize := origInfo.Size()

	res := h.Run("update")
	res.AssertOK(t)

	newInfo, err := os.Stat(stagingBin)
	require.NoError(t, err, "binary should still exist after atomic swap")

	assert.NotEqual(t, origSize, newInfo.Size(),
		"binary size should differ after swap; orig=%d new=%d", origSize, newInfo.Size())
	assert.NotZero(t, newInfo.Mode().Perm()&0o100, "swapped binary must be executable")
}

// TestUpdate_AlreadyCurrentNoOps verifies that when the gh stub returns the
// same version we built with, update is a no-op. The binary is staged outside
// any Homebrew prefix so the managed-install short-circuit does not fire.
func TestUpdate_AlreadyCurrentNoOps(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	stageBinary(t, h, filepath.Join(h.Home, "bin", "ergo"))

	// "integration" matches our ldflag; update should report up-to-date.
	h.InstallGhStub(harness.GhStubOptions{
		LatestTag: "integration",
	})

	res := h.Run("update")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "already up to date")
}

// TestUpdate_HomebrewManagedDefers verifies that when the running binary
// resolves under a Homebrew prefix, self-update is skipped in favor of
// `brew upgrade`, leaving the binary untouched even though a newer release exists.
func TestUpdate_HomebrewManagedDefers(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	prefix := filepath.Join(h.Home, "brew")
	stagingBin := filepath.Join(prefix, "bin", "ergo")
	stageBinary(t, h, stagingBin)
	h.SetEnv("HOMEBREW_PREFIX", prefix)

	// A newer release is available, but the Homebrew check must win first.
	h.InstallGhStub(harness.GhStubOptions{LatestTag: "v999.0.0"})

	origInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)

	res := h.Run("update")
	res.AssertOK(t)
	assert.Contains(t, res.Combined, "brew upgrade ergo")

	newInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)
	assert.Equal(t, origInfo.Size(), newInfo.Size(),
		"Homebrew-managed binary must not be self-replaced")
}

// TestUpdate_ChecksumMismatchAborts verifies that a download whose SHA-256 does
// not match the release checksums file is rejected before the atomic rename,
// leaving the original binary in place.
func TestUpdate_ChecksumMismatchAborts(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	stagingBin := filepath.Join(h.Home, "bin", "ergo")
	stageBinary(t, h, stagingBin)

	h.InstallGhStub(harness.GhStubOptions{
		LatestTag:       "v999.0.0",
		CorruptChecksum: true,
	})

	origInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)

	res := h.Run("update")
	res.AssertFail(t)
	assert.Contains(t, res.Combined, "checksum")

	newInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)
	assert.Equal(t, origInfo.Size(), newInfo.Size(),
		"binary must not be replaced when the checksum does not match")
}
