//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// TestList_EmptyAndPopulated covers both the no-workspaces hint and the table
// rendering after seeding workspace TOMLs directly (init has no non-interactive
// shorthand; we exercise it via direct seeding plus list/validate).
func TestList_EmptyAndPopulated(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	// Empty case: list should print the helpful hint.
	res := h.Run("list")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "no workspaces defined")

	// Seed two workspaces and verify both show up.
	h.WriteWorkspaceTOML("alpha", `
[workspace]
name = "alpha"

[[repos]]
url = "https://example.com/x.git"
`)
	h.WriteWorkspaceTOML("beta", `
[workspace]
name = "beta"
`)

	res = h.Run("list")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "alpha")
	assert.Contains(t, res.Stdout, "beta")
	assert.Contains(t, res.Stdout, "not synced")
}

// TestValidate_HappyPath confirms that a syntactically valid TOML passes.
func TestValidate_HappyPath(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	h.WriteWorkspaceTOML("good", `
[workspace]
name = "good"

[[repos]]
url = "https://example.com/foo.git"

[[repos]]
url = "https://example.com/bar.git"
`)

	res := h.Run("validate", "good")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "good")
}

// TestValidate_DetectsCollision exercises the duplicate-derived-name validation.
func TestValidate_DetectsCollision(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	h.WriteWorkspaceTOML("collide", `
[workspace]
name = "collide"

[[repos]]
url = "https://example.com/owner-a/utils.git"

[[repos]]
url = "https://example.com/owner-b/utils.git"
`)

	res := h.Run("validate", "collide")
	res.AssertFail(t)
	require.NotZero(t, res.ExitCode)

	combined := strings.ToLower(res.Combined)
	assert.Contains(t, combined, "utils")
}

// TestValidate_AllFlag iterates over every workspace and aggregates failures.
func TestValidate_AllFlag(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	h.WriteWorkspaceTOML("ok", `
[workspace]
name = "ok"

[[repos]]
url = "https://example.com/foo.git"
`)
	h.WriteWorkspaceTOML("broken", `
[workspace]
name = "broken"

[[repos]]
url = ""
`)

	res := h.Run("validate", "--all")
	res.AssertFail(t)
	assert.Contains(t, res.Stdout, "ok")
	assert.Contains(t, res.Stdout, "broken")
}
