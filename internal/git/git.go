package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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

// Clone clones repoURL into destDir. If branch is non-empty, the given branch
// is checked out; otherwise git uses the remote's default branch.
func Clone(r Runner, repoURL, destDir, branch string) error {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, destDir)

	if _, err := r.Run("", "git", args...); err != nil {
		return fmt.Errorf("cloning %s: %w", repoURL, err)
	}
	return nil
}

// Pull fetches and fast-forward pulls the current branch in dir.
func Pull(r Runner, dir string) error {
	if _, err := r.Run(dir, "git", "pull", "--ff-only"); err != nil {
		return fmt.Errorf("pulling in %s: %w", dir, err)
	}
	return nil
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
