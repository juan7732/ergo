package git

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runnerFunc adapts a plain function to the Runner interface.
type runnerFunc func(dir, name string, args ...string) (string, error)

func (f runnerFunc) Run(dir, name string, args ...string) (string, error) {
	return f(dir, name, args...)
}

// captureRunner records the most recent call and returns a fixed response.
type captureRunner struct {
	dir  string
	name string
	args []string
	out  string
	err  error
}

func (c *captureRunner) Run(dir, name string, args ...string) (string, error) {
	c.dir = dir
	c.name = name
	c.args = args
	return c.out, c.err
}

func TestClone_CallsGitCloneWithBranch(t *testing.T) {
	r := &captureRunner{}
	err := Clone(r, "https://github.com/example/repo.git", "/tmp/repo", "main")
	require.NoError(t, err)
	assert.Equal(t, "git", r.name)
	assert.Equal(t, []string{"clone", "--branch", "main", "https://github.com/example/repo.git", "/tmp/repo"}, r.args)
}

func TestClone_OmitsBranchWhenEmpty(t *testing.T) {
	r := &captureRunner{}
	err := Clone(r, "https://github.com/example/repo.git", "/tmp/repo", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"clone", "https://github.com/example/repo.git", "/tmp/repo"}, r.args)
}

func TestClone_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("permission denied")}
	err := Clone(r, "https://github.com/example/repo.git", "/tmp/repo", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloning https://github.com/example/repo.git")
}

// multiCallRunner records each Run call and returns scripted responses.
type multiCallRunner struct {
	calls   [][]string
	dirs    []string
	outs    []string
	errs    []error
	callIdx int
}

func (m *multiCallRunner) Run(dir, name string, args ...string) (string, error) {
	m.calls = append(m.calls, append([]string{name}, args...))
	m.dirs = append(m.dirs, dir)
	i := m.callIdx
	m.callIdx++
	var out string
	var err error
	if i < len(m.outs) {
		out = m.outs[i]
	}
	if i < len(m.errs) {
		err = m.errs[i]
	}
	return out, err
}

func TestClone_RetriesWithoutBranchWhenRemoteBranchMissing(t *testing.T) {
	dest := t.TempDir() + "/repo"
	// Simulate git's behavior of creating the dest directory before failing.
	require.NoError(t, os.MkdirAll(dest, 0o700))

	r := &multiCallRunner{
		errs: []error{
			errors.New("fatal: Remote branch main not found in upstream origin: exit status 128"),
			nil,
		},
	}
	err := Clone(r, "https://github.com/example/repo.git", dest, "main")
	require.NoError(t, err)
	require.Len(t, r.calls, 2)
	assert.Equal(t, []string{"git", "clone", "--branch", "main", "https://github.com/example/repo.git", dest}, r.calls[0])
	assert.Equal(t, []string{"git", "clone", "https://github.com/example/repo.git", dest}, r.calls[1])

	// The partial destination directory should have been removed before retry.
	// (It is recreated by the retry in real git, but our fake runner does not,
	// so the dir should not exist now.)
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "expected dest dir to be cleaned up, got %v", statErr)
}

func TestClone_DoesNotRetryOnUnrelatedError(t *testing.T) {
	r := &multiCallRunner{
		errs: []error{errors.New("fatal: repository not found")},
	}
	err := Clone(r, "https://github.com/example/repo.git", "/tmp/repo", "main")
	require.Error(t, err)
	assert.Len(t, r.calls, 1)
	assert.Contains(t, err.Error(), "cloning https://github.com/example/repo.git")
}

func TestPull_CallsGitPullFastForward(t *testing.T) {
	r := &captureRunner{}
	err := Pull(r, "/tmp/repo")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/repo", r.dir)
	assert.Equal(t, "git", r.name)
	assert.Equal(t, []string{"pull", "--ff-only"}, r.args)
}

func TestPull_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("not a git repo")}
	err := Pull(r, "/tmp/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulling in /tmp/repo")
}

func TestPull_ReturnsErrEmptyRemoteWhenUpstreamRefMissing(t *testing.T) {
	r := &captureRunner{err: errors.New("Your configuration specifies to merge with the ref 'refs/heads/main' from the remote, but no such ref was fetched.: exit status 1")}
	err := Pull(r, "/tmp/repo")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEmptyRemote), "expected ErrEmptyRemote, got %v", err)
	assert.Contains(t, err.Error(), "pulling in /tmp/repo")
}

func TestPull_ReturnsErrEmptyRemoteWhenRemoteRefNotFound(t *testing.T) {
	r := &captureRunner{err: errors.New("fatal: couldn't find remote ref refs/heads/main")}
	err := Pull(r, "/tmp/repo")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEmptyRemote), "expected ErrEmptyRemote, got %v", err)
}

func TestInit_CallsGitInit(t *testing.T) {
	r := &captureRunner{}
	err := Init(r, "/tmp/folder")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/folder", r.dir)
	assert.Equal(t, []string{"init"}, r.args)
}

func TestStatus_DirtyRepo(t *testing.T) {
	r := runnerFunc(func(dir, name string, args ...string) (string, error) {
		return " M cmd/root.go\n", nil
	})
	dirty, err := Status(r, "/tmp/repo")
	require.NoError(t, err)
	assert.True(t, dirty)
}

func TestStatus_CleanRepo(t *testing.T) {
	r := runnerFunc(func(dir, name string, args ...string) (string, error) {
		return "", nil
	})
	dirty, err := Status(r, "/tmp/repo")
	require.NoError(t, err)
	assert.False(t, dirty)
}

func TestStatus_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("not a git repository")}
	_, err := Status(r, "/tmp/not-a-repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking status in /tmp/not-a-repo")
}

func TestBehindCount_ParsesOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{"zero behind", "0", 0},
		{"three behind", "3", 3},
		{"with whitespace", "  5\n", 5},
		{"empty output", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runnerFunc(func(dir, name string, args ...string) (string, error) {
				return tt.output, nil
			})
			got, err := BehindCount(r, "/tmp/repo")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBehindCount_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("no upstream")}
	_, err := BehindCount(r, "/tmp/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "counting behind commits in /tmp/repo")
}

func TestCurrentBranch_ReturnsOutput(t *testing.T) {
	r := runnerFunc(func(dir, name string, args ...string) (string, error) {
		return "feat/tui\n", nil
	})
	branch, err := CurrentBranch(r, "/tmp/repo")
	require.NoError(t, err)
	assert.Equal(t, "feat/tui", branch)
}

func TestCurrentBranch_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("fatal: not a git repository")}
	_, err := CurrentBranch(r, "/tmp/not-a-repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting current branch in /tmp/not-a-repo")
}

func TestRepoRoot_ReturnsOutput(t *testing.T) {
	r := runnerFunc(func(dir, name string, args ...string) (string, error) {
		return "/home/user/my-repo\n", nil
	})
	root, err := RepoRoot(r, "/home/user/my-repo/pkg")
	require.NoError(t, err)
	assert.Equal(t, "/home/user/my-repo", root)
}

func TestRepoRoot_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("fatal: not a git repository")}
	_, err := RepoRoot(r, "/tmp/not-a-repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finding git root in /tmp/not-a-repo")
}
