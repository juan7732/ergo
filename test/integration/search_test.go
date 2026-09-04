//go:build integration

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juan7732/ergo/test/integration/harness"
)

// searchDoc mirrors the ergo search --json contract. Kind-specific fields
// are pointers so a test can assert they are absent for the other kinds.
type searchDoc struct {
	Query   string `json:"query"`
	Results []struct {
		Workspace string    `json:"workspace"`
		Kind      string    `json:"kind"`
		Name      string    `json:"name"`
		URL       *string   `json:"url"`
		Group     *string   `json:"group"`
		Tags      *[]string `json:"tags"`
		Cloned    *bool     `json:"cloned"`
		Created   *bool     `json:"created"`
		Synced    *bool     `json:"synced"`
		Path      string    `json:"path"`
	} `json:"results"`
}

// resultKey identifies one result as workspace/kind/name.
func (d searchDoc) byKey() map[string]int {
	m := map[string]int{}
	for i, r := range d.Results {
		m[r.Workspace+"/"+r.Kind+"/"+r.Name] = i
	}
	return m
}

// seedSearchFixtures writes three workspaces that between them exercise every
// match target: "alpha" and "beta" both reference the same "ergo" repo, beta
// also has an "ergo-notes" folder, and "ergo-lab" matches by workspace name
// only. Returns the seeded repo URL.
func seedSearchFixtures(t *testing.T, h *harness.Harness) string {
	t.Helper()
	repo := h.SeedBareRepo("ergo", map[string]string{"x.txt": "1\n"})
	other := h.SeedBareRepo("unrelated", map[string]string{"y.txt": "1\n"})

	h.WriteWorkspaceTOML("alpha", fmt.Sprintf(`
[workspace]
name = "alpha"

[[repos]]
url = %q
group = "core"
tags = ["go"]

[[repos]]
url = %q
`, repo, other))

	h.WriteWorkspaceTOML("beta", fmt.Sprintf(`
[workspace]
name = "beta"

[[repos]]
url = %q

[[folders]]
name = "ergo-notes"
`, repo))

	h.WriteWorkspaceTOML("ergo-lab", fmt.Sprintf(`
[workspace]
name = "ergo-lab"

[[repos]]
url = %q
`, other))
	return repo
}

func TestSearchJSON_CrossWorkspaceHitsAndDiskState(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	repo := seedSearchFixtures(t, h)

	res := h.Run("search", "ergo", "--json")
	res.AssertOK(t)
	var doc searchDoc
	mustParseJSON(t, res.Stdout, &doc)
	assert.Equal(t, "ergo", doc.Query)
	require.Len(t, doc.Results, 4)

	// Ordered by workspace, then kind (workspace, repo, folder), then name.
	assert.Equal(t, "alpha/repo/ergo", doc.Results[0].Workspace+"/"+doc.Results[0].Kind+"/"+doc.Results[0].Name)
	assert.Equal(t, "beta/repo/ergo", doc.Results[1].Workspace+"/"+doc.Results[1].Kind+"/"+doc.Results[1].Name)
	assert.Equal(t, "beta/folder/ergo-notes", doc.Results[2].Workspace+"/"+doc.Results[2].Kind+"/"+doc.Results[2].Name)
	assert.Equal(t, "ergo-lab/workspace/ergo-lab", doc.Results[3].Workspace+"/"+doc.Results[3].Kind+"/"+doc.Results[3].Name)

	keys := doc.byKey()

	alpha := doc.Results[keys["alpha/repo/ergo"]]
	require.NotNil(t, alpha.URL)
	assert.Equal(t, repo, *alpha.URL)
	require.NotNil(t, alpha.Group)
	assert.Equal(t, "core", *alpha.Group)
	require.NotNil(t, alpha.Tags)
	assert.Equal(t, []string{"go"}, *alpha.Tags)
	require.NotNil(t, alpha.Cloned)
	assert.False(t, *alpha.Cloned, "nothing materialized yet")
	assert.Nil(t, alpha.Created, "repo results carry no folder fields")
	assert.Nil(t, alpha.Synced, "repo results carry no workspace fields")
	assert.Equal(t, filepath.Join(h.WorkspaceDir("alpha"), "ergo"), alpha.Path)

	beta := doc.Results[keys["beta/repo/ergo"]]
	require.NotNil(t, beta.Group)
	assert.Equal(t, "", *beta.Group, "group is \"\" when unset, not omitted")
	require.NotNil(t, beta.Tags)
	assert.Equal(t, []string{}, *beta.Tags, "tags is [] when unset, not omitted")

	folder := doc.Results[keys["beta/folder/ergo-notes"]]
	require.NotNil(t, folder.Created)
	assert.False(t, *folder.Created)
	assert.Nil(t, folder.URL)
	assert.Nil(t, folder.Tags)
	assert.Nil(t, folder.Cloned)
	assert.Equal(t, filepath.Join(h.WorkspaceDir("beta"), "ergo-notes"), folder.Path)

	lab := doc.Results[keys["ergo-lab/workspace/ergo-lab"]]
	require.NotNil(t, lab.Synced)
	assert.False(t, *lab.Synced)
	assert.Nil(t, lab.Cloned)
	assert.Nil(t, lab.Created)
	assert.Equal(t, h.WorkspaceDir("ergo-lab"), lab.Path)

	// Materialize beta only: its repo becomes cloned and its folder created,
	// while alpha's copy of the same repo stays uncloned.
	h.Run("sync", "beta").AssertOK(t)

	res = h.Run("search", "ergo", "--json")
	res.AssertOK(t)
	doc = searchDoc{}
	mustParseJSON(t, res.Stdout, &doc)
	require.Len(t, doc.Results, 4)
	keys = doc.byKey()
	assert.False(t, *doc.Results[keys["alpha/repo/ergo"]].Cloned)
	assert.True(t, *doc.Results[keys["beta/repo/ergo"]].Cloned)
	assert.True(t, *doc.Results[keys["beta/folder/ergo-notes"]].Created)
	assert.False(t, *doc.Results[keys["ergo-lab/workspace/ergo-lab"]].Synced)

	// A query that only appears in repo URLs returns repos only.
	res = h.Run("search", "git-fixtures", "--json")
	res.AssertOK(t)
	doc = searchDoc{}
	mustParseJSON(t, res.Stdout, &doc)
	require.Len(t, doc.Results, 4)
	for _, r := range doc.Results {
		assert.Equal(t, "repo", r.Kind)
	}

	// Case-insensitive.
	res = h.Run("search", "ERGO", "--json")
	res.AssertOK(t)
	doc = searchDoc{}
	mustParseJSON(t, res.Stdout, &doc)
	assert.Len(t, doc.Results, 4)
}

func TestSearch_HumanTable(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	seedSearchFixtures(t, h)

	res := h.Run("search", "ergo")
	res.AssertOK(t)
	for _, want := range []string{"Workspace", "Kind", "State", "alpha", "beta", "ergo-lab", "ergo-notes", "uncloned", "not created", "not synced", "core", "go"} {
		assert.Contains(t, res.Stdout, want)
	}
	assert.NotContains(t, res.Stdout, "unrelated")

	h.Run("sync", "alpha").AssertOK(t)
	res = h.Run("search", "ergo")
	res.AssertOK(t)
	assert.Contains(t, res.Stdout, "cloned")
	assert.Contains(t, res.Stdout, "uncloned", "beta's copy is still uncloned")
}

func TestSearch_CorruptWorkspaceWarnsAndContinues(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	seedSearchFixtures(t, h)
	h.WriteWorkspaceTOML("broken", "[[[ this is not toml\n")

	res := h.Run("search", "ergo", "--json")
	res.AssertOK(t)
	var doc searchDoc
	mustParseJSON(t, res.Stdout, &doc)
	assert.Len(t, doc.Results, 4, "hits in healthy workspaces are unaffected")
	assert.Contains(t, res.Stderr, "warning: skipping broken")
	assert.NotContains(t, res.Stdout, "warning")

	res = h.Run("search", "ergo")
	res.AssertOK(t)
	assert.Contains(t, res.Stderr, "warning: skipping broken")
}

func TestSearch_NoMatchExitsZero(t *testing.T) {
	t.Parallel()
	h := harness.New(t)
	seedSearchFixtures(t, h)

	res := h.Run("search", "does-not-exist")
	res.AssertOK(t)
	assert.Equal(t, "no matches for \"does-not-exist\"\n", res.Stdout)

	res = h.Run("search", "does-not-exist", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"query\": \"does-not-exist\",\n  \"results\": []\n}\n", res.Stdout)

	// With no workspaces defined at all the same shape comes back.
	empty := harness.New(t)
	res = empty.Run("search", "ergo", "--json")
	res.AssertOK(t)
	assert.Equal(t, "{\n  \"query\": \"ergo\",\n  \"results\": []\n}\n", res.Stdout)
}

func TestSearch_ArgValidation(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	h.Run("search").AssertFail(t)
	h.Run("search", "a", "b").AssertFail(t)
	h.Run("search", "").AssertFail(t)
	h.Run("search", "   ").AssertFail(t)
}
