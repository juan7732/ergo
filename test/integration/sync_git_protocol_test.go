//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/juan7732/ergo/test/integration/harness"
)

// TestSync_SSHProtocolPassesThroughNonHTTPSURLs verifies that with
// [git] protocol = "ssh", non-https URLs (here file://) are passed to git
// unchanged and sync succeeds end-to-end.
func TestSync_SSHProtocolPassesThroughNonHTTPSURLs(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	repo := h.SeedBareRepo("gamma", map[string]string{"README.md": "# gamma\n"})

	h.WriteGlobalConfig(`
[defaults]
workspace_root = "~/ergo-workspaces"
default_branch = "main"

[sync]
auto_pull = true

[git]
protocol = "ssh"
`)

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
`, repo))

	res := h.Run("sync", "ws")
	res.AssertOK(t)
	assert.DirExists(t, filepath.Join(h.WorkspaceDir("ws"), "gamma"))
	assert.True(t, harness.IsGitRepo(filepath.Join(h.WorkspaceDir("ws"), "gamma")))
}

// TestSync_AuthFailureReportsHintInsteadOfHanging installs a git stub that
// mimics git's fail-fast output under GIT_TERMINAL_PROMPT=0 and asserts that
// sync surfaces a per-repo error carrying the [git] protocol hint.
func TestSync_AuthFailureReportsHintInsteadOfHanging(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	// Stub git: clone fails like an HTTPS repo needing credentials with
	// terminal prompts disabled; everything else succeeds silently.
	script := `#!/usr/bin/env bash
if [[ "${1:-}" == "clone" ]]; then
  echo "fatal: could not read Username for 'https://github.com': terminal prompts disabled" >&2
  exit 128
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(h.PathDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	h.WriteWorkspaceTOML("ws", `
[workspace]
name = "ws"

[[repos]]
url = "https://github.com/private/repo.git"
`)

	res := h.Run("sync", "ws")
	res.AssertFail(t)
	assert.Contains(t, res.Combined, "✗")
	assert.Contains(t, res.Combined, `protocol = "ssh"`)
	assert.Contains(t, res.Combined, "[git]")
}
