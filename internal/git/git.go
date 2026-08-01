package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ErrEmptyRemote is returned by Pull when the remote has no commits / no
// matching upstream ref (typical for freshly-created GitHub repos that have
// not received their first push yet). Callers should treat this as a no-op
// rather than a failure.
var ErrEmptyRemote = errors.New("remote is empty or missing upstream ref")

// Runner executes a shell command in a given directory and returns trimmed stdout.
// The thin interface exists solely to enable test fakes without spawning real git
// processes — do not add methods here until a second implementation exists.
type Runner interface {
	Run(dir, name string, args ...string) (string, error)
}

// ExecRunner is the default Runner that shells out via os/exec.
type ExecRunner struct{}

// Run executes name with args in dir and returns trimmed stdout.
// Stderr is included in the error message when the command fails.
func (ExecRunner) Run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	// GIT_TERMINAL_PROMPT=0 makes git fail fast instead of opening /dev/tty to
	// prompt for credentials — prompts would hang or interleave during parallel
	// sync. Only ergo-initiated git commands run through ExecRunner; user
	// commands from `ergo run` use workspace.runInDir and are unaffected.
	// GIT_ASKPASS is deliberately left alone so non-interactive credential
	// helpers (VS Code, git-credential-manager) keep working.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%s: %w", msg, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CheckPath returns an error if git is not found on PATH.
func CheckPath() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: install git and retry")
	}
	return nil
}

// authHint returns a one-line remediation hint for authentication failures,
// or "" when err does not look auth-related.
//
// DECISION: hint detection lives here in the git package, next to the error
// text it matches, so every caller (sync, open) benefits without touching
// result rendering. Hints are single-line because sync progress output renders
// each repo error on one line.
func authHint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "terminal prompts disabled"),
		strings.Contains(msg, "could not read username"),
		strings.Contains(msg, "could not read password"),
		strings.Contains(msg, "authentication failed"):
		return `(hint: git needs credentials for HTTPS; if you use SSH keys, set protocol = "ssh" under [git] in ~/.ergo/config.toml)`
	case strings.Contains(msg, "permission denied (publickey"):
		return `(hint: SSH auth failed; check ssh-agent and your key, or set protocol = "https" under [git] in ~/.ergo/config.toml)`
	}
	return ""
}

// withAuthHint appends the auth remediation hint to err's message when it
// looks like an authentication failure, preserving the wrapped error.
func withAuthHint(err error) error {
	if h := authHint(err); h != "" {
		return fmt.Errorf("%w %s", err, h)
	}
	return err
}

// Clone clones repoURL into destDir. If branch is non-empty, the given branch
// is checked out; otherwise git uses the remote's default branch.
//
// When --branch is requested but the remote does not have that branch (e.g. an
// empty repo with no commits yet, or a repo whose default branch differs from
// the configured one), Clone removes the partial destination and retries
// without --branch so the clone falls back to the remote's default.
func Clone(r Runner, repoURL, destDir, branch string) error {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, destDir)

	_, err := r.Run("", "git", args...)
	if err == nil {
		return nil
	}

	// Retry without --branch when the remote lacks the requested branch.
	// This is common for newly-created empty repos that have no branches yet.
	if branch != "" && isRemoteBranchMissing(err) {
		if rmErr := os.RemoveAll(destDir); rmErr != nil {
			return fmt.Errorf("cleaning up after failed clone of %s: %w", repoURL, rmErr)
		}
		retryArgs := []string{"clone", repoURL, destDir}
		if _, retryErr := r.Run("", "git", retryArgs...); retryErr != nil {
			return withAuthHint(fmt.Errorf("cloning %s (retry without --branch): %w", repoURL, retryErr))
		}
		return nil
	}

	return withAuthHint(fmt.Errorf("cloning %s: %w", repoURL, err))
}

// isRemoteBranchMissing reports whether err looks like git's "Remote branch X
// not found in upstream origin" failure from `git clone --branch`.
func isRemoteBranchMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "remote branch") && strings.Contains(msg, "not found")
}

// Pull fetches and fast-forward pulls the current branch in dir.
//
// Returns ErrEmptyRemote (wrapped) when git fails because the configured
// upstream ref does not exist on the remote — typical for freshly-created
// repos that have no commits yet. Callers should treat this as a no-op.
func Pull(r Runner, dir string) error {
	if _, err := r.Run(dir, "git", "pull", "--ff-only"); err != nil {
		if isMissingUpstreamRef(err) {
			return fmt.Errorf("pulling in %s: %w", dir, ErrEmptyRemote)
		}
		return withAuthHint(fmt.Errorf("pulling in %s: %w", dir, err))
	}
	return nil
}

// isMissingUpstreamRef reports whether err looks like git's "no such ref was
// fetched" failure from `git pull` against an empty remote (the local branch
// is configured to merge with refs/heads/<X>, but the remote has no <X>).
func isMissingUpstreamRef(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such ref was fetched") ||
		(strings.Contains(msg, "couldn't find remote ref") && strings.Contains(msg, "refs/heads/"))
}

// Init runs git init in dir.
func Init(r Runner, dir string) error {
	if _, err := r.Run(dir, "git", "init"); err != nil {
		return fmt.Errorf("initializing git repo in %s: %w", dir, err)
	}
	return nil
}

// Status returns true when the repo at dir has uncommitted changes (dirty).
// It uses --porcelain so the output is stable across git versions.
func Status(r Runner, dir string) (bool, error) {
	out, err := r.Run(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("checking status in %s: %w", dir, err)
	}
	return strings.TrimSpace(out) != "", nil
}

// BehindCount returns the number of commits the local branch is behind its
// upstream (git rev-list --count HEAD..@{u}).
// Returns 0 when the output is empty (e.g. no upstream configured).
func BehindCount(r Runner, dir string) (int, error) {
	out, err := r.Run(dir, "git", "rev-list", "--count", "HEAD..@{u}")
	if err != nil {
		return 0, fmt.Errorf("counting behind commits in %s: %w", dir, err)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("parsing behind count %q in %s: %w", out, dir, err)
	}
	return n, nil
}

// CurrentBranch returns the name of the current branch in dir.
func CurrentBranch(r Runner, dir string) (string, error) {
	out, err := r.Run(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch in %s: %w", dir, err)
	}
	return strings.TrimSpace(out), nil
}

// RepoRoot returns the absolute path of the git repository root containing dir.
// Returns an error if dir is not inside a git repository.
func RepoRoot(r Runner, dir string) (string, error) {
	out, err := r.Run(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("finding git root in %s: %w", dir, err)
	}
	return strings.TrimSpace(out), nil
}
