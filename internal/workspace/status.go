package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/juan7732/ergo/internal/config"
	"github.com/juan7732/ergo/internal/git"
)

// RepoStatusEntry holds the observed state of a single repo.
type RepoStatusEntry struct {
	// Name is the repo's directory name on disk.
	Name string
	// Branch is the current checked-out branch. Empty when Uncloned is true.
	Branch string
	// Dirty is true when there are uncommitted changes.
	Dirty bool
	// Behind is the number of commits the local branch is behind its upstream.
	// 0 when Uncloned is true or no upstream is configured.
	Behind int
	// Uncloned is true when the repo directory does not exist on disk.
	Uncloned bool
	// Group is the logical group from the TOML (empty string if unset).
	Group string
}

// GatherStatus gathers status for all repos defined in cfg.
// wsDir is the absolute path to the workspace directory on disk.
// When parallel is true, up to batchSize repos are queried concurrently.
// Individual git errors are absorbed — each entry reflects best-effort state
// (e.g. missing upstream just results in Behind == 0).
func GatherStatus(cfg config.WorkspaceConfig, wsDir string, r git.Runner, parallel bool, batchSize int) ([]RepoStatusEntry, error) {
	if batchSize <= 0 {
		batchSize = 1
	}

	entries := make([]RepoStatusEntry, len(cfg.Repos))
	for i, repo := range cfg.Repos {
		entries[i] = RepoStatusEntry{Name: repo.EffectiveName(), Group: repo.Group}
	}

	if !parallel || batchSize == 1 || len(cfg.Repos) == 0 {
		for i, repo := range cfg.Repos {
			dir := filepath.Join(wsDir, repo.EffectiveName())
			entries[i] = gatherRepoStatus(repo.EffectiveName(), repo.Group, dir, r)
		}
		return entries, nil
	}

	// Parallel path: bounded concurrency via semaphore.
	sem := make(chan struct{}, batchSize)
	var mu sync.Mutex

	g, _ := errgroup.WithContext(context.Background())
	for i, repo := range cfg.Repos {
		i, repo := i, repo
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := filepath.Join(wsDir, repo.EffectiveName())
			entry := gatherRepoStatus(repo.EffectiveName(), repo.Group, dir, r)

			mu.Lock()
			entries[i] = entry
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("gathering status: %w", err)
	}
	return entries, nil
}

// GatherSingleRepoStatus gathers status for a standalone repo at dir.
// name is the display name and group is optional (pass empty string for standalone repos).
func GatherSingleRepoStatus(dir, name, group string, r git.Runner) RepoStatusEntry {
	return gatherRepoStatus(name, group, dir, r)
}

// gatherRepoStatus collects the status of a single repo directory.
// Git errors are absorbed: missing upstream → Behind 0, etc.
func gatherRepoStatus(name, group, dir string, r git.Runner) RepoStatusEntry {
	entry := RepoStatusEntry{Name: name, Group: group}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		entry.Uncloned = true
		return entry
	}

	if branch, err := git.CurrentBranch(r, dir); err == nil {
		entry.Branch = branch
	}

	if dirty, err := git.Status(r, dir); err == nil {
		entry.Dirty = dirty
	}

	if behind, err := git.BehindCount(r, dir); err == nil {
		entry.Behind = behind
	}

	return entry
}
