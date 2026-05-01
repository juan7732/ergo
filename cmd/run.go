package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/tui"
	"juan7732/ergo/internal/workspace"
)

var runCmd = &cobra.Command{
	Use:   "run [workspace-name] -- <command> [args...]",
	Short: "Execute a command across all repos in the workspace",
	Long: `Execute a command across all (or filtered) repos in the workspace.

The command to run must be preceded by '--' to separate it from ergo flags:

  ergo run -- git status
  ergo run --tags=go -- go test ./...
  ergo run ml-projects -- git pull

Group exclusion: repos in groups listed in [run].excluded_groups (global config)
are automatically skipped. Override with --include-group or --all.

When run inside a standalone git repo (outside any ergo workspace), the command
runs in that repo only — filter flags and group exclusion are ignored.`,
	Args: cobra.ArbitraryArgs,
	RunE: runRun,
}

func init() {
	runCmd.Flags().Bool("fail-fast", false, "Stop after the first non-zero exit code")
	runCmd.Flags().Bool("include-folders", false, "Also run in non-repo [[folders]]")
	runCmd.Flags().String("include-group", "", "Override group exclusion for this specific group")
	runCmd.Flags().Bool("all", false, "Include all repos, ignoring excluded_groups")
	runCmd.Flags().String("name", "", "Filter repos by name (glob pattern, case-insensitive)")
	runCmd.Flags().String("group", "", "Filter repos to this group")
	runCmd.Flags().StringSlice("tags", nil, "Filter repos by tags, any-match (comma-separated or repeated flag)")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	includeFolders, _ := cmd.Flags().GetBool("include-folders")
	includeGroup, _ := cmd.Flags().GetString("include-group")
	all, _ := cmd.Flags().GetBool("all")

	// Split args at the '--' separator into [workspace-name] and command.
	dashIdx := cmd.ArgsLenAtDash()
	if dashIdx < 0 {
		return fmt.Errorf("missing command separator; use: ergo run [workspace] -- <command> [args...]")
	}

	var nameArg string
	if dashIdx > 0 {
		nameArg = args[0]
	}
	command := args[dashIdx:]

	if len(command) == 0 {
		return fmt.Errorf("no command specified; use: ergo run [workspace] -- <command> [args...]")
	}

	out := cmd.OutOrStdout()

	// Standalone repo fallback: when no explicit workspace and CWD is in a git repo.
	if nameArg == "" {
		cwd, err := currentDir()
		if err != nil {
			return err
		}
		det, err := workspace.Detect(cwd, execRunner())
		if err != nil {
			return fmt.Errorf("detecting workspace: %w", err)
		}

		if det.IsStandaloneRepo {
			// SPEC: filter flags and group exclusion are ignored in standalone mode.
			target := workspace.RunTarget{
				Name: filepath.Base(det.StandaloneRepoRoot),
				Dir:  det.StandaloneRepoRoot,
			}
			runOpts := workspace.RunOptions{
				Command:  command,
				Parallel: false,
				FailFast: failFast,
				OnResult: func(res workspace.RunResult) {
					tui.PrintRunResult(out, res)
				},
			}
			results, err := workspace.RunAcrossTargets([]workspace.RunTarget{target}, runOpts)
			if err != nil {
				return err
			}
			return runExitSummary(results)
		}
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsDir, err := workspaceDir(globalCfg, name)
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	filterOpts, err := filterOptsFromFlags(cmd, globalCfg.Run.ExcludedGroups)
	if err != nil {
		return err
	}
	filterOpts.IncludeGroup = includeGroup
	filterOpts.All = all

	filteredRepos := workspace.ApplyRepoFilter(wsCfg.Repos, filterOpts)

	// Build run targets from filtered repos, and optionally folders.
	targets := make([]workspace.RunTarget, 0, len(filteredRepos)+len(wsCfg.Folders))
	for _, repo := range filteredRepos {
		targets = append(targets, workspace.RunTarget{
			Name: repo.EffectiveName(),
			Dir:  filepath.Join(wsDir, repo.EffectiveName()),
		})
	}
	if includeFolders {
		for _, folder := range wsCfg.Folders {
			targets = append(targets, workspace.RunTarget{
				Name: folder.Name,
				Dir:  filepath.Join(wsDir, folder.Name),
			})
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(out, "no repos matched the filter")
		return nil
	}

	runOpts := workspace.RunOptions{
		Command:   command,
		Parallel:  globalCfg.Parallel.Enabled,
		BatchSize: globalCfg.Parallel.BatchSize,
		FailFast:  failFast,
		OnResult: func(res workspace.RunResult) {
			tui.PrintRunResult(out, res)
		},
	}

	results, err := workspace.RunAcrossTargets(targets, runOpts)
	if err != nil {
		return err
	}
	return runExitSummary(results)
}

// runExitSummary returns an error when any result had a non-zero exit code or
// infrastructure error. This surfaces a non-zero CLI exit code to the caller.
func runExitSummary(results []workspace.RunResult) error {
	var failCount int
	for _, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			failCount++
		}
	}
	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed", failCount)
	}
	return nil
}
