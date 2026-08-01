package workspace

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/juan7732/ergo/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRunner implements git.Runner and records every call, safe for the
// parallel sync path.
type recordingRunner struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *recordingRunner) Run(_, name string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string{name}, args...))
	return "", nil
}

func (r *recordingRunner) cloneURLs(t *testing.T) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var urls []string
	for _, call := range r.calls {
		// [git clone <url> <dest>] — no --branch in these tests.
		if len(call) >= 3 && call[1] == "clone" {
			urls = append(urls, call[2])
		}
	}
	return urls
}

func syncSingleRepo(t *testing.T, url string, opts SyncOptions) *recordingRunner {
	t.Helper()
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "test"},
		Repos:     []config.Repo{{URL: url}},
	}
	opts.WorkspaceDir = t.TempDir()
	r := &recordingRunner{}
	_, err := Sync(cfg, opts, r)
	require.NoError(t, err)
	return r
}

func TestSync_RewritesCloneURLWhenSSHEnabled(t *testing.T) {
	r := syncSingleRepo(t, "https://github.com/o/r.git", SyncOptions{RewriteURLsToSSH: true})
	assert.Equal(t, []string{"git@github.com:o/r.git"}, r.cloneURLs(t))
}

func TestSync_KeepsCloneURLWhenSSHDisabled(t *testing.T) {
	r := syncSingleRepo(t, "https://github.com/o/r.git", SyncOptions{RewriteURLsToSSH: false})
	assert.Equal(t, []string{"https://github.com/o/r.git"}, r.cloneURLs(t))
}

func TestSync_PullPathUnaffectedByRewrite(t *testing.T) {
	cfg := config.WorkspaceConfig{
		Workspace: config.WorkspaceMeta{Name: "test"},
		Repos:     []config.Repo{{URL: "https://github.com/o/r.git"}},
	}
	wsDir := t.TempDir()
	// Pre-create the repo directory so sync takes the pull path, not clone.
	require.NoError(t, os.MkdirAll(filepath.Join(wsDir, "r"), 0o700))

	r := &recordingRunner{}
	_, err := Sync(cfg, SyncOptions{
		WorkspaceDir:     wsDir,
		AutoPull:         true,
		RewriteURLsToSSH: true,
	}, r)
	require.NoError(t, err)

	assert.Empty(t, r.cloneURLs(t))
	require.Len(t, r.calls, 1)
	assert.Equal(t, []string{"git", "pull", "--ff-only"}, r.calls[0])
}
