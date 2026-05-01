package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"

	"golang.org/x/sync/errgroup"
)

// RunTarget is a named directory in which a command will be executed.
type RunTarget struct {
	// Name is the display name (repo or folder name) shown in output headers.
	Name string
	// Dir is the absolute path of the directory to execute the command in.
	Dir string
}

// RunResult holds the result of running a command in a single target directory.
type RunResult struct {
	// Name is the target's display name.
	Name string
	// Output is the combined stdout and stderr from the command.
	Output string
	// ExitCode is the command's exit code (0 = success).
	ExitCode int
	// Err is non-nil for infrastructure errors (e.g. binary not found, dir missing).
	// A non-zero exit code does not set Err.
	Err error
}

// RunOptions configures a cross-target run.
type RunOptions struct {
	// Command is the command and its arguments (must be non-empty).
	Command []string
	// Parallel enables concurrent execution when true.
	Parallel bool
	// BatchSize caps concurrent operations. Clamped to 1 when ≤ 0.
	BatchSize int
	// FailFast stops after the first target with a non-zero exit code or Err.
	FailFast bool
	// OnResult is called (thread-safe) immediately after each target finishes.
	// May be nil.
	OnResult func(RunResult)
}

// RunAcrossTargets executes opts.Command in each target's directory.
// Returns all results collected up to any fail-fast stop.
func RunAcrossTargets(targets []RunTarget, opts RunOptions) ([]RunResult, error) {
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	if !opts.Parallel || batchSize == 1 {
		var results []RunResult
		for _, t := range targets {
			res := runInDir(t.Name, t.Dir, opts.Command)
			results = append(results, res)
			if opts.OnResult != nil {
				opts.OnResult(res)
			}
			if opts.FailFast && (res.ExitCode != 0 || res.Err != nil) {
				return results, nil
			}
		}
		return results, nil
	}

	// Parallel path: bounded concurrency + optional fail-fast via context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultSlice := make([]RunResult, len(targets))
	sem := make(chan struct{}, batchSize)
	var mu sync.Mutex
	halted := false

	g, _ := errgroup.WithContext(ctx)
	for i, t := range targets {
		i, t := i, t
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			stop := halted && opts.FailFast
			mu.Unlock()
			if stop {
				return nil
			}

			res := runInDir(t.Name, t.Dir, opts.Command)

			mu.Lock()
			resultSlice[i] = res
			if opts.FailFast && (res.ExitCode != 0 || res.Err != nil) {
				halted = true
				cancel()
			}
			mu.Unlock()

			if opts.OnResult != nil {
				opts.OnResult(res)
			}
			return nil
		})
	}

	_ = g.Wait()

	// Collect results that were actually populated (fail-fast may leave some empty).
	var results []RunResult
	for _, r := range resultSlice {
		if r.Name != "" {
			results = append(results, r)
		}
	}
	return results, nil
}

// runInDir executes command in dir, capturing combined stdout+stderr.
// A non-zero exit code is recorded in ExitCode, not returned as Err.
// Err is set only for infrastructure failures (binary not found, dir missing, etc.).
func runInDir(name, dir string, command []string) RunResult {
	// #nosec G204 — executing arbitrary user-specified commands is ergo run's purpose.
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	runErr := cmd.Run()
	if runErr == nil {
		return RunResult{Name: name, Output: combined.String(), ExitCode: 0}
	}

	exitErr, ok := runErr.(*exec.ExitError)
	if !ok {
		// Infrastructure error: binary not found, directory missing, etc.
		return RunResult{Name: name, Err: fmt.Errorf("running command in %s: %w", dir, runErr)}
	}

	return RunResult{
		Name:     name,
		Output:   combined.String(),
		ExitCode: exitErr.ExitCode(),
	}
}
