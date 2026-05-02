//go:build integration

// Package harness provides utilities for running ergo end-to-end tests
// against the real binary in a hermetic sandbox.
package harness

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// defaultErgoBinary is the path to the ergo binary inside the container image.
// The Dockerfile copies the build artifact here. Override with $ERGO_TEST_BINARY
// to run the integration suite against a binary at a different path (useful
// when iterating outside the container).
const defaultErgoBinary = "/usr/local/bin/ergo"

// ErgoBinary returns the path to the ergo binary the harness should exec.
func ErgoBinary() string {
	if v := os.Getenv("ERGO_TEST_BINARY"); v != "" {
		return v
	}
	return defaultErgoBinary
}

// Harness is a per-test sandbox: an isolated HOME, a captured PATH, and
// helpers to write/read workspace state.
type Harness struct {
	t       *testing.T
	Home    string   // value of $HOME for this test (also $ERGO_HOME parent)
	PathDir string   // writable dir prepended to PATH for stub binaries
	Env     []string // env passed to ergo invocations (HOME, PATH, plus extras)
}

// New creates a new Harness with a fresh temp HOME and an empty PATH-prefix dir.
// Sandboxing is per-test (uses t.TempDir).
func New(t *testing.T) *Harness {
	t.Helper()

	home := t.TempDir()
	pathDir := filepath.Join(home, ".bin")
	require.NoError(t, os.MkdirAll(pathDir, 0o755))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ergo", "workspaces"), 0o700))

	h := &Harness{
		t:       t,
		Home:    home,
		PathDir: pathDir,
	}
	h.Env = h.baseEnv()
	return h
}

// baseEnv returns a minimal env: HOME pointed at the sandbox, PATH prepending
// the sandbox stub dir to the inherited PATH.
func (h *Harness) baseEnv() []string {
	parentPath := os.Getenv("PATH")
	env := []string{
		"HOME=" + h.Home,
		"PATH=" + h.PathDir + ":" + parentPath,
		// Avoid color/TUI output that might confuse string assertions.
		"NO_COLOR=1",
		"TERM=dumb",
	}
	// Preserve a few git-relevant vars so the runtime image's git config works.
	for _, k := range []string{"LANG", "LC_ALL", "USER"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// SetEnv appends or overrides an env var for subsequent ergo invocations.
func (h *Harness) SetEnv(key, value string) {
	prefix := key + "="
	for i, e := range h.Env {
		if strings.HasPrefix(e, prefix) {
			h.Env[i] = prefix + value
			return
		}
	}
	h.Env = append(h.Env, prefix+value)
}

// Result is the outcome of a single ergo invocation.
type Result struct {
	Stdout   string
	Stderr   string
	Combined string // stdout + stderr in order written
	ExitCode int
	Err      error // non-nil only if exec itself failed (not on non-zero exit)
}

// RunOpts controls a single ergo invocation.
type RunOpts struct {
	Cwd     string        // working directory; defaults to Harness.Home
	Stdin   string        // optional stdin payload (e.g. "y\n" for confirmations)
	Timeout time.Duration // defaults to 60s
}

// Run invokes the ergo binary with the given args using default options.
func (h *Harness) Run(args ...string) Result {
	return h.RunWith(RunOpts{}, args...)
}

// RunIn invokes ergo with the given working directory and args.
func (h *Harness) RunIn(cwd string, args ...string) Result {
	return h.RunWith(RunOpts{Cwd: cwd}, args...)
}

// RunWith invokes ergo with the supplied options and args.
func (h *Harness) RunWith(opts RunOpts, args ...string) Result {
	h.t.Helper()

	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.Cwd == "" {
		opts.Cwd = h.Home
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ErgoBinary(), args...)
	cmd.Env = append([]string{}, h.Env...)
	cmd.Dir = opts.Cwd

	var combined bytes.Buffer
	stdout := &capturingWriter{tee: &combined}
	stderr := &capturingWriter{tee: &combined}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	err := cmd.Run()
	res := Result{
		Stdout:   stdout.buf.String(),
		Stderr:   stderr.buf.String(),
		Combined: combined.String(),
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.Err = err
		}
	}
	return res
}

// AssertOK fails the test when the result has a non-zero exit code or an exec error.
func (r Result) AssertOK(t *testing.T) {
	t.Helper()
	require.NoErrorf(t, r.Err, "ergo failed to start: %s", r.Combined)
	require.Equalf(t, 0, r.ExitCode, "ergo exited %d\n--- combined output ---\n%s", r.ExitCode, r.Combined)
}

// AssertFail fails the test when the result has a zero exit code.
func (r Result) AssertFail(t *testing.T) {
	t.Helper()
	require.NotEqualf(t, 0, r.ExitCode, "ergo unexpectedly succeeded\n--- combined output ---\n%s", r.Combined)
}

// capturingWriter buffers writes locally and also tees them into a shared buffer
// so we can reconstruct the interleaved combined output.
type capturingWriter struct {
	buf bytes.Buffer
	tee io.Writer
}

func (w *capturingWriter) Write(p []byte) (int, error) {
	_, _ = w.tee.Write(p)
	return w.buf.Write(p)
}

// ─── Workspace helpers ─────────────────────────────────────────────────────────

// WorkspaceTOMLPath returns the path to the workspace TOML file under HOME.
func (h *Harness) WorkspaceTOMLPath(name string) string {
	return filepath.Join(h.Home, ".ergo", "workspaces", name+".toml")
}

// WriteWorkspaceTOML writes the given TOML body to ~/.ergo/workspaces/<name>.toml.
func (h *Harness) WriteWorkspaceTOML(name, body string) {
	h.t.Helper()
	require.NoError(h.t, os.MkdirAll(filepath.Dir(h.WorkspaceTOMLPath(name)), 0o700))
	require.NoError(h.t, os.WriteFile(h.WorkspaceTOMLPath(name), []byte(body), 0o600))
}

// ReadWorkspaceTOML returns the contents of the workspace TOML file.
func (h *Harness) ReadWorkspaceTOML(name string) string {
	h.t.Helper()
	b, err := os.ReadFile(h.WorkspaceTOMLPath(name))
	require.NoError(h.t, err)
	return string(b)
}

// WriteGlobalConfig writes the given TOML body to ~/.ergo/config.toml.
func (h *Harness) WriteGlobalConfig(body string) {
	h.t.Helper()
	path := filepath.Join(h.Home, ".ergo", "config.toml")
	require.NoError(h.t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(h.t, os.WriteFile(path, []byte(body), 0o600))
}

// WorkspaceDir returns the materialized workspace directory path on disk
// (under the default ~/ergo-workspaces/ root).
func (h *Harness) WorkspaceDir(name string) string {
	return filepath.Join(h.Home, "ergo-workspaces", name)
}

// CodeWorkspaceFile returns the path to the generated <name>.code-workspace file.
func (h *Harness) CodeWorkspaceFile(name string) string {
	return filepath.Join(h.WorkspaceDir(name), name+".code-workspace")
}

// ReadCodeWorkspace reads the generated .code-workspace file and returns its bytes.
func (h *Harness) ReadCodeWorkspace(name string) []byte {
	h.t.Helper()
	b, err := os.ReadFile(h.CodeWorkspaceFile(name))
	require.NoError(h.t, err)
	return b
}
