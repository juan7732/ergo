# Typical Workflows

End-to-end traces of common user journeys, with the underlying function calls
called out so you can trace from prompt to filesystem.

---

## 1. First-time workspace creation

```bash
$ ergo init ml-projects
# TUI: workspace name prefilled → loop repos → loop folders → confirm
$ ergo open ml-projects
# Materializes on disk, clones repos, creates folders, opens VS Code
```

Behind the scenes:

| Step                        | Code path                                                                  |
| --------------------------- | -------------------------------------------------------------------------- |
| Read `init` arg             | `cmd/init.go::runInit`                                                     |
| Wizard                      | `internal/tui/init_wizard.go::InitWizard`                                  |
| Write TOML                  | `config.WriteWorkspace(name, cfg)` → `~/.ergo/workspaces/ml-projects.toml` |
| `open`'s fast-path check    | `isWorkspaceCurrent(wsDir, wsFilePath, wsCfg)` → false (dir missing)       |
| Materialize                 | `workspace.Sync` with `AutoPull=false`                                     |
| Per-repo clone              | `git.Clone(runner, url, dest, branch)`                                     |
| Per-folder mkdir / git init | `syncFolder`                                                               |
| Persist state               | `workspace.SaveState(state)` → `~/.ergo/state/ml-projects.json`            |
| Generate workspace file     | `vscode.Generate(cfg, nil)` + `vscode.WriteIfChanged(...)`                 |
| Launch editor               | `exec.Command("code", wsFilePath).Run()`                                   |

---

## 2. Daily sync

```bash
$ ergo sync           # CWD inside ~/ergo-workspaces/ml-projects/
```

Detection sequence (when no `nameArg`):

1. `workspace.Detect(cwd, runner)` first walks up looking for a
   `*.code-workspace` containing an `ergo.workspace-name` JSON key.
2. If none found, checks if `cwd` is under `<workspace_root>/<name>/`.
3. If still not found, considers it a standalone repo (or none).

Once detected:

- `config.LoadGlobal()` to pick up `[parallel]`/`[sync]`.
- `config.LoadWorkspace(name)`.
- `workspace.ApplyRepoFilter(...)` if any filter flags were given (no-op otherwise).
- `workspace.Sync(syncCfg, opts, runner)` — bounded-parallel
  clone/pull, sequential folder processing, orphan scan.
- `workspace.SaveState(state)` after the sync.
- `vscode.ReadFilter(...)` recovers any active `show` filter from the existing
  file; `vscode.Generate(...)` + `vscode.WriteIfChanged(...)` then regenerate
  the `.code-workspace` (through the filter, if one was active) only if the
  bytes changed. A note line surfaces the preserved filter — see
  [07-operational-semantics.md](07-operational-semantics.md#show-filter-preservation).
- Orphans are reported (and optionally deleted with `--force`).

---

## 3. Running a command across the workspace

```bash
$ ergo run --tags=go -- go test ./...
```

Path through `cmd/run.go::runRun`:

1. `cmd.ArgsLenAtDash()` finds the `--` boundary.
2. No workspace name → `workspace.Detect(cwd, runner)`.
3. If standalone repo: build a single `RunTarget`, call `RunAcrossTargets`,
   filter flags ignored.
4. Otherwise resolve workspace and load both configs.
5. `filterOpts := filterOptsFromFlags(cmd, globalCfg.Run.ExcludedGroups)`.
6. `workspace.ApplyRepoFilter(wsCfg.Repos, filterOpts)` — because `--tags`
   is explicit, `ExcludedGroups` is **not** applied.
7. Build `RunTarget`s from filtered repos (and `[[folders]]` if `--include-folders`).
8. `workspace.RunAcrossTargets(targets, runOpts)` with parallel execution.
9. Each finished result is printed via `tui.PrintRunResult` immediately.
10. Final exit: non-zero if any target had a non-zero exit code or `Err`.

---

## 4. Focusing Copilot on one slice

```bash
$ ergo show ml      # filter to group=ml
# work happens with reduced VS Code folder list
$ ergo show all     # restore full view
```

`cmd/show.go::runShow`:

- Always uses `workspace.Detect(cwd)` — there is no workspace-name argument
  for `show`. The positional arg is the **group** name (or `"all"`).
- `vscode.Filter{Group, Tags, Name}` is recorded in the regenerated
  `.code-workspace` under `ergo.filter`. Other tools that read the file can
  see what view is active.
- Folders and the root folder are **always** included; the filter affects
  repos only.
- TOML is never modified.

The state is "carried" by the file itself: re-running `ergo show ml` on a
workspace that already has the filter results in `WriteIfChanged` returning
`(false, nil)` and the user seeing `"filter already set to group ml"`.

The filter genuinely persists across the daily workflow: `sync` and `open`
read it back (`vscode.ReadFilter`) and re-apply it when they regenerate the
file, printing a note line while it is active. Hidden repos are still synced —
the filter only shapes the VS Code view
([07-operational-semantics.md](07-operational-semantics.md#show-filter-preservation)).
Tooling can query the active filter without touching the file semantics via
`ergo show --json` (`filter` is `null` when none is active).

---

## 5. Self-update

```bash
$ ergo update
```

`cmd/update.go::runUpdate`:

1. `github.CheckPath()` — abort if `gh` not on PATH.
2. `github.LatestRelease(runner)` — `gh release list --repo juan7732/ergo --limit 1 ...`.
3. Compare normalized tags (strip `v`). `dev` always proceeds.
4. `os.Executable()` + `EvalSymlinks` to find the **real** binary path.
5. `github.DownloadRelease(runner, tag, "ergo-darwin-arm64", exeDir)` →
   downloads alongside the running binary.
6. `os.Chmod(downloaded, 0o755)`.
7. `os.Rename(downloaded, exePath)` — atomic on the same filesystem.
8. Deferred cleanup removes the downloaded artifact on any failure.

---

## 6. Outside any workspace

```bash
$ cd ~/some-other-repo
$ ergo status
$ ergo run -- git status
```

Both commands detect a standalone git repo via `git rev-parse --show-toplevel`
and operate on just that repo. Filter flags and `excluded_groups` do not apply
in this mode.

`ergo list` and `ergo init` work identically inside or outside a workspace
because they don't need one.

For commands that *do* require a workspace (`open`, `sync`, `add`, `remove`,
`edit`, `validate`, `show` to a lesser extent), running outside a workspace
without an argument launches the `WorkspaceSelect` TUI.

---

## 7. Adding a repo and immediately syncing

```bash
$ ergo add repo https://github.com/juan/new.git --tags=go --group=tools
added repo "new" to workspace "ml-projects"
sync workspace now? [y/N] y
syncing workspace "ml-projects" → ~/ergo-workspaces/ml-projects
  ✓ new                          cloned
  ...
```

`promptSync` reads stdin only when `isTerminal()` — scripts piping into ergo
get a clean exit without the prompt.
