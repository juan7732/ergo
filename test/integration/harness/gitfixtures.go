//go:build integration

package harness

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// FixturesRoot returns the directory under HOME where bare git fixtures live.
func (h *Harness) FixturesRoot() string {
	return filepath.Join(h.Home, "git-fixtures")
}

// SeedBareRepo creates a bare git repository named <name>.git under the harness
// fixtures directory, populated with the given files (path → contents) in a
// single initial commit on the default branch (main).
//
// Returns a file:// URL suitable for use as a [[repos]] url in a workspace TOML.
func (h *Harness) SeedBareRepo(name string, files map[string]string) string {
	h.t.Helper()

	require.NoError(h.t, os.MkdirAll(h.FixturesRoot(), 0o755))

	// Working tree for the initial commit.
	work := filepath.Join(h.Home, ".seed", name)
	require.NoError(h.t, os.MkdirAll(work, 0o755))

	for relPath, content := range files {
		full := filepath.Join(work, relPath)
		require.NoError(h.t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(h.t, os.WriteFile(full, []byte(content), 0o644))
	}
	if len(files) == 0 {
		// Ensure at least one file so the initial commit isn't empty.
		require.NoError(h.t, os.WriteFile(filepath.Join(work, "README.md"), []byte("# "+name+"\n"), 0o644))
	}

	h.git(work, "init", "-q", "-b", "main")
	h.git(work, "add", "-A")
	h.git(work, "commit", "-q", "-m", "initial")

	bare := filepath.Join(h.FixturesRoot(), name+".git")
	h.git("", "clone", "-q", "--bare", work, bare)

	return "file://" + bare
}

// MutateBareRepo clones the bare repo, applies the mutation function to the
// working tree, commits the result, and pushes back to the bare repo. Use this
// to make a workspace clone "behind" by adding new commits upstream.
func (h *Harness) MutateBareRepo(repoURL string, mutate func(workDir string)) {
	h.t.Helper()

	work := h.t.TempDir()
	h.git("", "clone", "-q", repoURL, work)
	mutate(work)
	h.git(work, "add", "-A")
	h.git(work, "commit", "-q", "-m", "upstream change")
	h.git(work, "push", "-q", "origin", "main")
}

// git runs a git subcommand in dir and fails the test on error. dir may be empty.
func (h *Harness) git(dir string, args ...string) {
	h.t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ergo-test",
		"GIT_AUTHOR_EMAIL=ergo-test@example.com",
		"GIT_COMMITTER_NAME=ergo-test",
		"GIT_COMMITTER_EMAIL=ergo-test@example.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(h.t, err, "git %v failed: %s", args, string(out))
}

// IsGitRepo reports whether the given directory contains a .git entry.
func IsGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

// GitIn runs `git <args...>` in dir and fails the test on error. Useful from
// tests that need to manipulate a workspace clone (e.g. fetch to refresh
// remote-tracking refs before asserting behind-counts).
func (h *Harness) GitIn(dir string, args ...string) {
	h.git(dir, args...)
}
