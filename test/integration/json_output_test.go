//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// statusDoc mirrors the ergo status --json contract.
type statusDoc struct {
	Workspace string `json:"workspace"`
	Repos     []struct {
		Name     string   `json:"name"`
		Branch   string   `json:"branch"`
		Dirty    bool     `json:"dirty"`
		Behind   int      `json:"behind"`
		Uncloned bool     `json:"uncloned"`
		Group    string   `json:"group"`
		Tags     []string `json:"tags"`
	} `json:"repos"`
}

// mustParseJSON asserts stdout is exactly one clean JSON document (no prose,
// no ANSI) and decodes it into v.
func mustParseJSON(t *testing.T, stdout string, v any) {
	t.Helper()
	require.True(t, strings.HasPrefix(stdout, "{"), "stdout must start with the JSON document, got:\n%s", stdout)
	require.True(t, strings.HasSuffix(stdout, "}\n"), "stdout must end with the JSON document, got:\n%s", stdout)
	require.NotContains(t, stdout, "\x1b[", "stdout must not contain ANSI escapes")
	require.NoError(t, json.Unmarshal([]byte(stdout), v), "stdout is not valid JSON:\n%s", stdout)
}

func TestStatusJSON_WorkspaceMode(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	mlRepo := h.SeedBareRepo("ml-a", map[string]string{"x.txt": "1\n"})
	toolsRepo := h.SeedBareRepo("tools-b", map[string]string{"y.txt": "1\n"})
	absentRepo := h.SeedBareRepo("absent", map[string]string{"z.txt": "1\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
group = "ml"
tags = ["ml", "python"]

[[repos]]
url = %q
group = "tools"

[[repos]]
url = %q
`, mlRepo, toolsRepo, absentRepo))

	h.Run("open", "ws").AssertOK(t)
	wsDir := h.WorkspaceDir("ws")
	require.NoError(t, os.RemoveAll(filepath.Join(wsDir, "absent")))

	res := h.Run("status", "ws", "--json")
	res.AssertOK(t)

	var doc statusDoc
	mustParseJSON(t, res.Stdout, &doc)

	assert.Equal(t, "ws", doc.Workspace)
	require.Len(t, doc.Repos, 3)

	byName := map[string]int{}
	for i, r := range doc.Repos {
		byName[r.Name] = i
	}

	ml := doc.Repos[byName["ml-a"]]
	assert.NotEmpty(t, ml.Branch)
	assert.False(t, ml.Dirty)
	assert.False(t, ml.Uncloned)
	assert.Equal(t, "ml", ml.Group)
	assert.Equal(t, []string{"ml", "python"}, ml.Tags)

	tools := doc.Repos[byName["tools-b"]]
	assert.Equal(t, "tools", tools.Group)
	assert.Equal(t, []string{}, tools.Tags, "tags must be [] when none are defined, not null")

	abs := doc.Repos[byName["absent"]]
	assert.True(t, abs.Uncloned)
	assert.Equal(t, "", abs.Branch, "uncloned repos report an empty branch")
	assert.False(t, abs.Dirty)
	assert.Equal(t, 0, abs.Behind)

	// Filter flags compose with --json: the repos array reflects the filter.
	res = h.Run("status", "ws", "--json", "--group=ml")
	res.AssertOK(t)
	var filtered statusDoc
	mustParseJSON(t, res.Stdout, &filtered)
	require.Len(t, filtered.Repos, 1)
	assert.Equal(t, "ml-a", filtered.Repos[0].Name)

	// A filter matching nothing yields "repos": [] with exit 0.
	res = h.Run("status", "ws", "--json", "--group=nope")
	res.AssertOK(t)
	var empty statusDoc
	mustParseJSON(t, res.Stdout, &empty)
	assert.Equal(t, "ws", empty.Workspace)
	require.NotNil(t, empty.Repos)
	assert.Len(t, empty.Repos, 0)
}

func TestStatusJSON_StandaloneRepoMode(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	bare := h.SeedBareRepo("solo", map[string]string{"x.txt": "1\n"})
	codeDir := filepath.Join(h.Home, "code")
	require.NoError(t, os.MkdirAll(codeDir, 0o755))
	h.GitIn(codeDir, "clone", "-q", bare, "solo")

	res := h.RunIn(filepath.Join(codeDir, "solo"), "status", "--json")
	res.AssertOK(t)

	var doc statusDoc
	mustParseJSON(t, res.Stdout, &doc)

	assert.Equal(t, "", doc.Workspace, "standalone mode reports an empty workspace name")
	require.Len(t, doc.Repos, 1)
	assert.Equal(t, "solo", doc.Repos[0].Name)
	assert.False(t, doc.Repos[0].Uncloned)
	assert.Equal(t, "", doc.Repos[0].Group)
	assert.Equal(t, []string{}, doc.Repos[0].Tags)
}

func TestListJSON(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	type listDoc struct {
		Workspaces []struct {
			Name   string `json:"name"`
			Repos  int    `json:"repos"`
			Synced bool   `json:"synced"`
		} `json:"workspaces"`
	}

	// Empty state: {"workspaces": []}, exit 0 — not the human hint text.
	res := h.Run("list", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspaces\": []\n}\n", res.Stdout)

	repo := h.SeedBareRepo("r", map[string]string{"x.txt": "1\n"})
	h.WriteWorkspaceTOML("ws", fmt.Sprintf("[workspace]\nname = \"ws\"\n\n[[repos]]\nurl = %q\n", repo))

	res = h.Run("list", "--json")
	res.AssertOK(t)
	var doc listDoc
	mustParseJSON(t, res.Stdout, &doc)
	require.Len(t, doc.Workspaces, 1)
	assert.Equal(t, "ws", doc.Workspaces[0].Name)
	assert.Equal(t, 1, doc.Workspaces[0].Repos)
	assert.False(t, doc.Workspaces[0].Synced, "not materialized yet")

	h.Run("open", "ws").AssertOK(t)

	res = h.Run("list", "--json")
	res.AssertOK(t)
	doc = listDoc{}
	mustParseJSON(t, res.Stdout, &doc)
	require.Len(t, doc.Workspaces, 1)
	assert.True(t, doc.Workspaces[0].Synced)
}

func TestValidateJSON(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	type validateDoc struct {
		Workspace string `json:"workspace"`
		Valid     bool   `json:"valid"`
		Errors    []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	type validateAllDoc struct {
		Workspaces []validateDoc `json:"workspaces"`
	}

	// --all with no workspaces: {"workspaces": []}, exit 0.
	res := h.Run("validate", "--all", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspaces\": []\n}\n", res.Stdout)

	h.WriteWorkspaceTOML("good", `
[workspace]
name = "good"

[[repos]]
url = "https://github.com/e/a.git"
`)
	// Two repos deriving the same name → validation error on repos[1].
	h.WriteWorkspaceTOML("bad", `
[workspace]
name = "bad"

[[repos]]
url = "https://github.com/one/utils.git"

[[repos]]
url = "https://github.com/two/utils.git"
`)
	// Not TOML at all → parse failure, not a validation failure.
	h.WriteWorkspaceTOML("broken", "[[[ this is not toml\n")

	res = h.Run("validate", "good", "--json")
	res.AssertOK(t)
	var good validateDoc
	mustParseJSON(t, res.Stdout, &good)
	assert.Equal(t, "good", good.Workspace)
	assert.True(t, good.Valid)
	require.NotNil(t, good.Errors, "errors must be present and [] when valid")
	assert.Len(t, good.Errors, 0)

	res = h.Run("validate", "bad", "--json")
	res.AssertFail(t)
	var bad validateDoc
	mustParseJSON(t, res.Stdout, &bad)
	assert.False(t, bad.Valid)
	require.Len(t, bad.Errors, 1)
	assert.Equal(t, "repos[1]", bad.Errors[0].Field)
	assert.Contains(t, bad.Errors[0].Message, `"utils"`)

	res = h.Run("validate", "broken", "--json")
	res.AssertFail(t)
	var broken validateDoc
	mustParseJSON(t, res.Stdout, &broken)
	assert.False(t, broken.Valid)
	require.Len(t, broken.Errors, 1)
	assert.Equal(t, "", broken.Errors[0].Field, "parse failures use an empty field")
	assert.NotEmpty(t, broken.Errors[0].Message)

	res = h.Run("validate", "--all", "--json")
	res.AssertFail(t)
	var all validateAllDoc
	mustParseJSON(t, res.Stdout, &all)
	require.Len(t, all.Workspaces, 3)
	valids := map[string]bool{}
	for _, w := range all.Workspaces {
		valids[w.Workspace] = w.Valid
	}
	assert.Equal(t, map[string]bool{"good": true, "bad": false, "broken": false}, valids)
}

func TestConfigCommand(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	type configDoc struct {
		Workspace string `json:"workspace"`
		Repos     []struct {
			Name  string   `json:"name"`
			URL   string   `json:"url"`
			Tags  []string `json:"tags"`
			Group string   `json:"group"`
		} `json:"repos"`
		Folders []struct {
			Name string `json:"name"`
			Git  bool   `json:"git"`
		} `json:"folders"`
	}

	toml := `
[workspace]
name = "ws"

[[repos]]
url = "https://github.com/juan/handwriting-recognition.git"
tags = ["ml", "python"]
group = "ml"

[[repos]]
url = "https://github.com/other-org/utils.git"
name = "utils-other"

[[folders]]
name = "scratch"

[[folders]]
name = "planning"
git = true
`
	h.WriteWorkspaceTOML("ws", toml)

	// Default form prints the TOML verbatim (no materialization needed).
	res := h.Run("config", "ws")
	res.AssertOK(t)
	assert.Equal(t, toml, res.Stdout)

	// --json normalizes: derived + explicit names, tags [] when unset.
	res = h.Run("config", "ws", "--json")
	res.AssertOK(t)
	var doc configDoc
	mustParseJSON(t, res.Stdout, &doc)

	assert.Equal(t, "ws", doc.Workspace)
	require.Len(t, doc.Repos, 2)
	assert.Equal(t, "handwriting-recognition", doc.Repos[0].Name, "name derived from URL")
	assert.Equal(t, "https://github.com/juan/handwriting-recognition.git", doc.Repos[0].URL)
	assert.Equal(t, []string{"ml", "python"}, doc.Repos[0].Tags)
	assert.Equal(t, "ml", doc.Repos[0].Group)
	assert.Equal(t, "utils-other", doc.Repos[1].Name, "explicit name wins")
	assert.Equal(t, []string{}, doc.Repos[1].Tags, "tags must be [], not null")
	assert.Equal(t, "", doc.Repos[1].Group)
	require.Len(t, doc.Folders, 2)
	assert.Equal(t, "scratch", doc.Folders[0].Name)
	assert.False(t, doc.Folders[0].Git)
	assert.Equal(t, "planning", doc.Folders[1].Name)
	assert.True(t, doc.Folders[1].Git)

	// Unknown workspace errors in both forms.
	h.Run("config", "nope").AssertFail(t)
	h.Run("config", "nope", "--json").AssertFail(t)
}

func TestShowJSON_ReadOnly(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	h.InstallCodeStub()

	mlRepo := h.SeedBareRepo("ml-a", map[string]string{"x.txt": "1\n"})
	toolsRepo := h.SeedBareRepo("tools-b", map[string]string{"y.txt": "1\n"})

	h.WriteWorkspaceTOML("ws", fmt.Sprintf(`
[workspace]
name = "ws"

[[repos]]
url = %q
group = "ml"
tags = ["go"]

[[repos]]
url = %q
group = "tools"
`, mlRepo, toolsRepo))

	h.Run("open", "ws").AssertOK(t)
	wsDir := h.WorkspaceDir("ws")

	// No filter active: filter is null. Exact golden bytes.
	res := h.RunIn(wsDir, "show", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspace\": \"ws\",\n  \"filter\": null\n}\n", res.Stdout)

	// Group filter active.
	h.RunIn(wsDir, "show", "ml").AssertOK(t)
	res = h.RunIn(wsDir, "show", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspace\": \"ws\",\n  \"filter\": {\n    \"group\": \"ml\"\n  }\n}\n", res.Stdout)

	// show --json must not have modified the workspace file.
	assert.Contains(t, string(h.ReadCodeWorkspace("ws")), `"group": "ml"`)

	// Tag filter active.
	h.RunIn(wsDir, "show", "--tag=go").AssertOK(t)
	res = h.RunIn(wsDir, "show", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspace\": \"ws\",\n  \"filter\": {\n    \"tags\": [\n      \"go\"\n    ]\n  }\n}\n", res.Stdout)

	// Mutating show + --json is rejected.
	h.RunIn(wsDir, "show", "ml", "--json").AssertFail(t)
	h.RunIn(wsDir, "show", "--tag=go", "--json").AssertFail(t)

	// Clearing restores null.
	h.RunIn(wsDir, "show", "all").AssertOK(t)
	res = h.RunIn(wsDir, "show", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"workspace\": \"ws\",\n  \"filter\": null\n}\n", res.Stdout)
}
