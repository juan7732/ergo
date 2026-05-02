//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"juan7732/ergo/test/integration/harness"
)

// seedRunWorkspace creates a 3-repo workspace with groups + tags useful for
// exercising filters and excluded_groups. Returns the workspace dir.
func seedRunWorkspace(t *testing.T, h *harness.Harness) string {
	t.Helper()
	h.InstallCodeStub()

	repoML := h.SeedBareRepo("ml-app", map[string]string{"x": "1\n"})
	repoTool := h.SeedBareRepo("tool-app", map[string]string{"x": "1\n"})
	repoDoc := h.SeedBareRepo("doc-site", map[string]string{"x": "1\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
group = "ml"
tags = ["python"]

[[repos]]
url = %q
group = "tools"
tags = ["go"]

[[repos]]
url = %q
group = "documentation"
tags = ["docs"]
`, repoML, repoTool, repoDoc))

	// Exclude "documentation" from `ergo run` by default.
	h.WriteGlobalConfig(`
[defaults]
workspace_root = "~/ergo-workspaces"
default_branch = "main"

[parallel]
enabled = true
batch_size = 4

[sync]
auto_pull = true

[run]
excluded_groups = ["documentation"]
`)

	h.Run("open", "ws").AssertOK(t)
	return h.WorkspaceDir("ws")
}

func TestRun_DefaultExcludesGroup(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	res := h.RunIn(wsDir, "run", "ws", "--", "git", "rev-parse", "--abbrev-ref", "HEAD")
	res.AssertOK(t)

	out := res.Combined
	assert.Contains(t, out, "ml-app")
	assert.Contains(t, out, "tool-app")
	assert.NotContains(t, out, "doc-site",
		"documentation group should be excluded by default; got:\n%s", out)
}

func TestRun_AllOverridesExclusion(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	res := h.RunIn(wsDir, "run", "ws", "--all", "--", "git", "rev-parse", "--abbrev-ref", "HEAD")
	res.AssertOK(t)
	assert.Contains(t, res.Combined, "doc-site")
}

func TestRun_TagsFilter(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	res := h.RunIn(wsDir, "run", "ws", "--tags=go", "--", "git", "rev-parse", "--abbrev-ref", "HEAD")
	res.AssertOK(t)

	out := res.Combined
	assert.Contains(t, out, "tool-app")
	assert.NotContains(t, out, "ml-app", "ml-app does not have the 'go' tag")
}

func TestRun_FailFastStopsAfterFirst(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	// `false` exits non-zero everywhere. With --fail-fast plus serial execution
	// this should exit non-zero, and we should see the error reported.
	// We disable parallelism via env tweak to global config so order is deterministic.
	h.WriteGlobalConfig(`
[defaults]
workspace_root = "~/ergo-workspaces"
default_branch = "main"

[parallel]
enabled = false
batch_size = 1

[sync]
auto_pull = true

[run]
excluded_groups = ["documentation"]
`)

	res := h.RunIn(wsDir, "run", "ws", "--fail-fast", "--", "false")
	res.AssertFail(t)
}

func TestShow_GroupRecordsFilter(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	res := h.RunIn(wsDir, "show", "ml")
	res.AssertOK(t)

	raw := h.ReadCodeWorkspace("ws")
	var parsed struct {
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
		Ergo map[string]any `json:"ergo"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))

	// Root + only the ml repo should remain.
	names := folderNames(parsed.Folders)
	assert.Contains(t, names, "root")
	assert.Contains(t, names, "ml-app")
	assert.NotContains(t, names, "tool-app")
	assert.NotContains(t, names, "doc-site")

	// Filter metadata recorded under the "ergo" object.
	require.NotNil(t, parsed.Ergo["filter"], "ergo.filter should be set when a filter is active")
	filter, ok := parsed.Ergo["filter"].(map[string]any)
	require.True(t, ok, "ergo.filter should decode as an object")
	assert.Equal(t, "ml", filter["group"])
}

func TestShow_AllClearsFilter(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	wsDir := seedRunWorkspace(t, h)

	h.RunIn(wsDir, "show", "ml").AssertOK(t)
	h.RunIn(wsDir, "show", "all").AssertOK(t)

	raw := h.ReadCodeWorkspace("ws")
	var parsed struct {
		Folders []struct {
			Name string `json:"name"`
		} `json:"folders"`
		Ergo map[string]any `json:"ergo"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))

	names := folderNames3(parsed.Folders)
	assert.Contains(t, names, "ml-app")
	assert.Contains(t, names, "tool-app")
	assert.Contains(t, names, "doc-site")

	if f, ok := parsed.Ergo["filter"]; ok {
		assert.Nil(t, f, "ergo.filter should be cleared after `show all`; got %v", f)
	}
}

func folderNames(fs []struct {
	Name string `json:"name"`
	Path string `json:"path"`
}) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

func folderNames3(fs []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}
