package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeScript writes a small shell script to dir/name and makes it executable.
func makeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	require.NoError(t, err)
	return path
}

func TestRunAcrossTargets_SingleTarget_Success(t *testing.T) {
	dir := t.TempDir()
	makeScript(t, dir, "myscript.sh", "echo hello")

	targets := []RunTarget{{Name: "repo", Dir: dir}}
	results, err := RunAcrossTargets(targets, RunOptions{
		Command: []string{"sh", "myscript.sh"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "repo", results[0].Name)
	assert.Contains(t, results[0].Output, "hello")
	assert.Equal(t, 0, results[0].ExitCode)
	assert.NoError(t, results[0].Err)
}

func TestRunAcrossTargets_NonZeroExitCode_RecordedNotErr(t *testing.T) {
	dir := t.TempDir()
	makeScript(t, dir, "fail.sh", "exit 42")

	targets := []RunTarget{{Name: "repo", Dir: dir}}
	results, err := RunAcrossTargets(targets, RunOptions{
		Command: []string{"sh", "fail.sh"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 42, results[0].ExitCode)
	assert.NoError(t, results[0].Err)
}

func TestRunAcrossTargets_InfrastructureError_SetsErr(t *testing.T) {
	dir := t.TempDir()
	targets := []RunTarget{{Name: "repo", Dir: dir}}
	results, err := RunAcrossTargets(targets, RunOptions{
		Command: []string{"__nonexistent_binary_ergo_test__"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Error(t, results[0].Err)
}

func TestRunAcrossTargets_FailFast_Sequential(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	makeScript(t, dir1, "run.sh", "exit 1")
	makeScript(t, dir2, "run.sh", "echo ran")

	targets := []RunTarget{
		{Name: "first", Dir: dir1},
		{Name: "second", Dir: dir2},
	}
	results, err := RunAcrossTargets(targets, RunOptions{
		Command:  []string{"sh", "run.sh"},
		FailFast: true,
	})
	require.NoError(t, err)
	// Only the first result should be present; fail-fast stopped after it.
	assert.Len(t, results, 1)
	assert.Equal(t, "first", results[0].Name)
}

func TestRunAcrossTargets_CombinedOutputCaptured(t *testing.T) {
	dir := t.TempDir()
	makeScript(t, dir, "both.sh", "echo stdout; echo stderr >&2")

	targets := []RunTarget{{Name: "repo", Dir: dir}}
	results, err := RunAcrossTargets(targets, RunOptions{
		Command: []string{"sh", "both.sh"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "stdout")
	assert.Contains(t, results[0].Output, "stderr")
}

func TestRunAcrossTargets_OnResultCalledForEach(t *testing.T) {
	dirs := make([]string, 3)
	targets := make([]RunTarget, 3)
	for i := range dirs {
		dirs[i] = t.TempDir()
		makeScript(t, dirs[i], "run.sh", fmt.Sprintf("echo repo%d", i))
		targets[i] = RunTarget{Name: fmt.Sprintf("repo%d", i), Dir: dirs[i]}
	}

	var called []string
	results, err := RunAcrossTargets(targets, RunOptions{
		Command: []string{"sh", "run.sh"},
		OnResult: func(r RunResult) {
			called = append(called, r.Name)
		},
	})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Len(t, called, 3)
}

func TestRunAcrossTargets_EmptyTargets_ReturnsEmpty(t *testing.T) {
	results, err := RunAcrossTargets(nil, RunOptions{Command: []string{"echo"}})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRunAcrossTargets_NoCommand_ReturnsError(t *testing.T) {
	_, err := RunAcrossTargets([]RunTarget{{Name: "x", Dir: "/tmp"}}, RunOptions{})
	assert.Error(t, err)
}

func TestRunAcrossTargets_Parallel_AllTargetsRun(t *testing.T) {
	dirs := make([]string, 5)
	targets := make([]RunTarget, 5)
	for i := range dirs {
		dirs[i] = t.TempDir()
		makeScript(t, dirs[i], "run.sh", fmt.Sprintf("echo %d", i))
		targets[i] = RunTarget{Name: fmt.Sprintf("r%d", i), Dir: dirs[i]}
	}

	results, err := RunAcrossTargets(targets, RunOptions{
		Command:   []string{"sh", "run.sh"},
		Parallel:  true,
		BatchSize: 3,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5)
}
