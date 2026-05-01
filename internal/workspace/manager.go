package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/git"
)

// RepoAction describes what Sync did with a single repo.
type RepoAction string

const (
	RepoActionCloned  RepoAction = "cloned"
	RepoActionPulled  RepoAction = "pulled"
	RepoActionSkipped RepoAction = "skipped"
	RepoActionFailed  RepoAction = "failed"
)

// FolderAction describes what Sync did with a single folder.
type FolderAction string

const (
	FolderActionCreated   FolderAction = "created"
	FolderActionGitInited FolderAction = "git-inited"
	FolderActionSkipped   FolderAction = "skipped"
	FolderActionFailed    FolderAction = "failed"
)

// RepoResult holds the outcome of syncing a single repo.
type RepoResult struct {
	Name   string
	Action RepoAction
	Err    error
}

// FolderResult holds the outcome of syncing a single folder.
type FolderResult struct {
	Name   string
	Action FolderAction
	Err    error
}

// SyncResult aggregates all per-repo and per-folder results from a Sync call.
type SyncResult struct {
	Repos   []RepoResult
	Folders []FolderResult
	// Orphans contains directory names in the workspace root that are not
	// referenced by any repo or folder in the TOML.
	Orphans []string
}

// SyncOptions configures a Sync call.
type SyncOptions struct {
	// WorkspaceDir is the absolute path to the workspace directory on disk.
	WorkspaceDir string
	// AutoPull controls whether an existing repo is pulled (fetch + pull) or
	// just skipped. Comes from [sync].auto_pull in global config.
	AutoPull bool
	// Parallel enables concurrent operations when true.
	Parallel bool
	// BatchSize caps the number of concurrent operations. Ignored when Parallel
	// is false. Clamped to 1 when ≤ 0.
	BatchSize int
	// Progress, when non-nil, is called after each repo completes (possibly from
	// multiple goroutines — callers must be thread-safe).
	Progress func(name string, action RepoAction, err error)
}

// Sync reconciles the workspace directory on disk with cfg.
//
// For each repo in cfg.Repos:
//   - Directory missing  → git clone
//   - Directory present, AutoPull true → git pull --ff-only
//   - Directory present, AutoPull false → skip
//
// For each folder in cfg.Folders:
//   - Directory missing → create (os.MkdirAll)
//   - Directory present with git=true and not yet a git repo → git init
//
// It always creates the workspace root directory if it does not exist.
// Sync never deletes anything; orphaned directories are reported in SyncResult.Orphans.
func Sync(cfg config.WorkspaceConfig, opts SyncOptions, r git.Runner) (SyncResult, error) {
	if err := os.MkdirAll(opts.WorkspaceDir, 0o700); err != nil {
		return SyncResult{}, fmt.Errorf("creating workspace directory %s: %w", opts.WorkspaceDir, err)
	}

	// Build a set of known names (repos + folders) for orphan detection.
	knownNames := make(map[string]struct{}, len(cfg.Repos)+len(cfg.Folders))

	repoResults := make([]RepoResult, len(cfg.Repos))
	for i, repo := range cfg.Repos {
		knownNames[repo.EffectiveName()] = struct{}{}
		repoResults[i] = RepoResult{Name: repo.EffectiveName()}
	}
	for _, folder := range cfg.Folders {
		knownNames[folder.Name] = struct{}{}
	}

	// Sync repos — potentially in parallel.
	if err := syncRepos(cfg.Repos, repoResults, opts, r); err != nil {
		return SyncResult{}, err
	}

	// Sync folders — always sequential (fast, no network I/O).
	folderResults := make([]FolderResult, 0, len(cfg.Folders))
	for _, folder := range cfg.Folders {
		fr := syncFolder(folder, opts.WorkspaceDir, r)
		folderResults = append(folderResults, fr)
	}

	// Detect orphans: entries in workspace dir not in TOML.
	orphans, err := findOrphans(opts.WorkspaceDir, knownNames)
	if err != nil {
		// Non-fatal: warn but don't fail the entire sync.
		fmt.Fprintf(os.Stderr, "warning: scanning for orphans: %v\n", err)
	}

	return SyncResult{
		Repos:   repoResults,
		Folders: folderResults,
		Orphans: orphans,
	}, nil
}

// syncRepos runs all repo operations, respecting parallel/batch settings.
func syncRepos(repos []config.Repo, results []RepoResult, opts SyncOptions, r git.Runner) error {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	if !opts.Parallel || batchSize == 1 || len(repos) == 0 {
		for i, repo := range repos {
			results[i] = syncRepo(repo, opts, r)
			if opts.Progress != nil {
				opts.Progress(results[i].Name, results[i].Action, results[i].Err)
			}
		}
		return nil
	}

	// Parallel path: use errgroup + semaphore for bounded concurrency.
	// Individual repo errors are stored in results[i].Err, not returned from
	// the goroutine, so errgroup.Wait() only returns infrastructure errors.
	sem := make(chan struct{}, batchSize)
	var mu sync.Mutex

	g, _ := errgroup.WithContext(context.Background())
	for i, repo := range repos {
		i, repo := i, repo // capture for goroutine
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			res := syncRepo(repo, opts, r)

			mu.Lock()
			results[i] = res
			mu.Unlock()

			if opts.Progress != nil {
				opts.Progress(res.Name, res.Action, res.Err)
			}
			return nil
		})
	}
	return g.Wait()
}

// syncRepo performs the clone-or-pull logic for a single repo.
func syncRepo(repo config.Repo, opts SyncOptions, r git.Runner) RepoResult {
	name := repo.EffectiveName()
	dir := filepath.Join(opts.WorkspaceDir, name)

	info, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return RepoResult{Name: name, Action: RepoActionFailed,
			Err: fmt.Errorf("checking repo directory %s: %w", dir, err)}
	}

	if os.IsNotExist(err) || info == nil {
		// Directory does not exist — clone.
		branch := ""
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		if cloneErr := git.Clone(r, repo.URL, dir, branch); cloneErr != nil {
			return RepoResult{Name: name, Action: RepoActionFailed, Err: cloneErr}
		}
		return RepoResult{Name: name, Action: RepoActionCloned}
	}

	// Directory exists.
	if !opts.AutoPull {
		return RepoResult{Name: name, Action: RepoActionSkipped}
	}

	if pullErr := git.Pull(r, dir); pullErr != nil {
		return RepoResult{Name: name, Action: RepoActionFailed, Err: pullErr}
	}
	return RepoResult{Name: name, Action: RepoActionPulled}
}

// syncFolder creates a folder and optionally runs git init.
func syncFolder(folder config.Folder, workspaceDir string, r git.Runner) FolderResult {
	dir := filepath.Join(workspaceDir, folder.Name)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return FolderResult{
			Name:   folder.Name,
			Action: FolderActionFailed,
			Err:    fmt.Errorf("creating folder %s: %w", dir, err),
		}
	}

	if !folder.Git {
		return FolderResult{Name: folder.Name, Action: FolderActionSkipped}
	}

	// Check if already a git repo.
	_, err := os.Stat(filepath.Join(dir, ".git"))
	if err == nil {
		// Already a git repo — nothing to do.
		return FolderResult{Name: folder.Name, Action: FolderActionSkipped}
	}

	if initErr := git.Init(r, dir); initErr != nil {
		return FolderResult{Name: folder.Name, Action: FolderActionFailed, Err: initErr}
	}
	return FolderResult{Name: folder.Name, Action: FolderActionGitInited}
}

// findOrphans returns directory names inside workspaceDir that are not present
// in knownNames.
func findOrphans(workspaceDir string, knownNames map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading workspace directory: %w", err)
	}

	var orphans []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, known := knownNames[name]; !known {
			orphans = append(orphans, name)
		}
	}
	return orphans, nil
}

// DeleteOrphan removes a single orphaned directory from the workspace.
// It is the caller's responsibility to confirm with the user before calling this.
func DeleteOrphan(workspaceDir, name string) error {
	dir := filepath.Join(workspaceDir, name)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("deleting orphan %s: %w", name, err)
	}
	return nil
}
