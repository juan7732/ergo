//go:build integration

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/juan7732/ergo/test/integration/harness"
)

// TestSync_FilteredForceDoesNotDeleteOutOfFilterRepos guards against the
// filtered-sync orphan bug: `ergo sync --group=<g> --force` must only delete
// directories that are absent from the *full* TOML, never repos that merely
// fall outside the active filter. Orphan detection is computed against the
// whole config; the filter only narrows the clone/pull operation set.
func TestSync_FilteredForceDoesNotDeleteOutOfFilterRepos(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	core := h.SeedBareRepo("core-svc", map[string]string{"a.txt": "a\n"})
	tools := h.SeedBareRepo("tools-cli", map[string]string{"b.txt": "b\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
group = "core"

[[repos]]
url = %q
group = "tools"
`, core, tools))

	// Materialize both repos.
	h.Run("open", "ws").AssertOK(t)
	wsDir := h.WorkspaceDir("ws")
	assert.DirExists(t, filepath.Join(wsDir, "core-svc"))
	assert.DirExists(t, filepath.Join(wsDir, "tools-cli"))

	// Sync only the "core" group, with --force and confirmation piped in.
	// tools-cli is outside the filter but IS in the TOML, so it must survive.
	res := h.RunWith(harness.RunOpts{Stdin: "y\n"}, "sync", "ws", "--group=core", "--force")
	res.AssertOK(t)

	assert.DirExists(t, filepath.Join(wsDir, "core-svc"), "filtered repo must remain")
	assert.DirExists(t, filepath.Join(wsDir, "tools-cli"),
		"out-of-filter repo is in the TOML and must NOT be treated as an orphan")
	assert.NotContains(t, res.Combined, "tools-cli",
		"out-of-filter repo must not be reported as an orphan")
}

// TestSync_ForceAndAddAreMutuallyExclusive guards the destructive-ordering bug:
// running with both --force (delete orphans) and --add (adopt orphans) is
// contradictory, so it must be rejected rather than deleting dirs the --add
// step would then try to re-adopt.
func TestSync_ForceAndAddAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	repo := h.SeedBareRepo("only", map[string]string{"x.txt": "x\n"})
	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
`, repo))
	h.Run("open", "ws").AssertOK(t)

	res := h.RunWith(harness.RunOpts{Stdin: "y\n"}, "sync", "ws", "--force", "--add")
	res.AssertFail(t)
}
