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

// TestUpdate_AtomicReplace stubs `gh` to serve a newer release and asserts that
// the `update` command replaces the binary at the path it was launched from.
//
// We don't run the post-swap binary (the asset is a placeholder, not a real ergo
// build); we only verify the swap happens.
func TestUpdate_AtomicReplace(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	// Stage: write a fake "ergo" binary at a writable location and point the
	// harness at it for this test only. Since `ergo update` does
	// os.Executable() + atomic rename, the swap targets this staged path
	// rather than the shared integration binary.
	stagingBin := filepath.Join(h.Home, "bin", "ergo")
	require.NoError(t, os.MkdirAll(filepath.Dir(stagingBin), 0o755))

	// Copy the real (integration-built) binary so `--version` / update logic works.
	src, err := os.ReadFile(harness.ErgoBinary())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(stagingBin, src, 0o755))
	h.Binary = stagingBin

	// Stub gh to advertise a newer release and serve a placeholder asset.
	// Use a tag that won't match the embedded "integration" version so update proceeds.
	h.InstallGhStub(harness.GhStubOptions{
		LatestTag: "v999.0.0",
	})

	// Capture original mtime of the staged binary.
	origInfo, err := os.Stat(stagingBin)
	require.NoError(t, err)
	origSize := origInfo.Size()

	res := h.Run("update")
	res.AssertOK(t)

	// Verify the binary on disk has been replaced.
	newInfo, err := os.Stat(stagingBin)
	require.NoError(t, err, "binary should still exist after atomic swap")

	assert.NotEqual(t, origSize, newInfo.Size(),
		"binary size should differ after swap; orig=%d new=%d", origSize, newInfo.Size())

	// Mode must be executable (0755).
	assert.NotZero(t, newInfo.Mode().Perm()&0o100, "swapped binary must be executable")
}

// TestUpdate_AlreadyCurrentNoOps verifies that when the gh stub returns the
// same version we built with, update is a no-op.
func TestUpdate_AlreadyCurrentNoOps(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	// "integration" matches our ldflag; update should report up-to-date.
	h.InstallGhStub(harness.GhStubOptions{
		LatestTag: "integration",
	})

	res := h.Run("update")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "already up to date")
}
