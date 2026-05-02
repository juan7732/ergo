//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"juan7732/ergo/test/integration/harness"
)

// TestStatus_CleanDirtyBehindUncloned exercises the four status states.
func TestStatus_CleanDirtyBehindUncloned(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	cleanRepo := h.SeedBareRepo("clean", map[string]string{"x.txt": "1\n"})
	dirtyRepo := h.SeedBareRepo("dirty", map[string]string{"x.txt": "1\n"})
	behindRepo := h.SeedBareRepo("behind", map[string]string{"x.txt": "1\n"})
	uncloneRepo := h.SeedBareRepo("absent", map[string]string{"x.txt": "1\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q

[[repos]]
url = %q

[[repos]]
url = %q

[[repos]]
url = %q
`, cleanRepo, dirtyRepo, behindRepo, uncloneRepo))

	h.Run("open", "ws").AssertOK(t)

	wsDir := h.WorkspaceDir("ws")

	// Make "dirty" actually dirty.
	a := assert.New(t)
	a.NoError(os.WriteFile(filepath.Join(wsDir, "dirty", "scratch.txt"), []byte("uncommitted\n"), 0o644))

	// Push a new commit upstream into "behind" so the workspace clone falls behind.
	h.MutateBareRepo(behindRepo, func(work string) {
		a.NoError(os.WriteFile(filepath.Join(work, "new.txt"), []byte("upstream\n"), 0o644))
	})
	// Refresh remote-tracking refs so `git rev-list --count HEAD..@{u}` reports >0.
	// ergo status doesn't fetch automatically.
	h.GitIn(filepath.Join(wsDir, "behind"), "fetch", "-q", "origin")

	// Remove "absent" from disk so it shows as uncloned.
	a.NoError(os.RemoveAll(filepath.Join(wsDir, "absent")))

	// Short output is easiest to assert against.
	res := h.Run("status", "ws", "--short")
	res.AssertOK(t)

	out := res.Stdout
	assertHasLine := func(repoName, expectedSubstr string) {
		t.Helper()
		var matched string
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.Contains(line, repoName) {
				matched = line
				break
			}
		}
		assert.NotEmpty(t, matched, "no status line for %q in:\n%s", repoName, out)
		assert.Contains(t, matched, expectedSubstr, "wrong status for %q (line: %q)", repoName, matched)
	}

	assertHasLine("clean", "clean")
	assertHasLine("dirty", "dirty")
	// "behind" reports a non-zero behind count.
	assertHasLine("behind", "1")
	assertHasLine("absent", "uncloned")
}
