# Internals — Package by Package

A deep tour of every internal package. Cross-reference with
[01-architecture.md](01-architecture.md) for the high-level diagram.

---

## `internal/config`

Files:
- [`types.go`](../../ergo/internal/config/types.go) — struct definitions.
- [`global.go`](../../ergo/internal/config/global.go) — `LoadGlobal`, default creation, `ErgoHome()`.
- [`workspace.go`](../../ergo/internal/config/workspace.go) — `LoadWorkspace`, `WriteWorkspace`, `ListWorkspaceNames`, `DeriveRepoName`, `ExpandTilde`.
- [`validate.go`](../../ergo/internal/config/validate.go) — `Validate`, `ValidationError`, `ValidationErrors`.

Important behaviors:

- **Defaults are lazy.** `LoadGlobal()` materializes `~/.ergo/config.toml` with
  the canonical defaults the first time it's called.
- **Pointer fields signal "unset".** `Repo.Name` and `Repo.Branch` are `*string`
  so the BurntSushi parser leaves them `nil` when omitted, allowing
  `EffectiveName()` and the sync logic to apply derivation/defaults correctly.
- **`WriteWorkspace` is full re-marshal.** No comment preservation in v1
  (per the implementation plan, "TOML round-trip writer (full re-marshal —
  comment preservation not in scope)").
- `ListWorkspaceNames()` returns names without the `.toml` extension and
  swallows `os.IsNotExist` (returns nil slice).

---

## `internal/git`

Files: [`git.go`](../../ergo/internal/git/git.go),
[`url.go`](../../ergo/internal/git/url.go)

Tiny adapter over the system `git` binary. The `Runner` interface is the
**only preemptive abstraction** in the codebase — it exists solely to enable
unit tests with fakes (see [`git_test.go`](../../ergo/internal/git/git_test.go)).

`ExecRunner.Run` executes any binary, captures stdout & stderr separately,
returns trimmed stdout, and on failure formats the stderr (or stdout if stderr
empty) into the error message. It sets `GIT_TERMINAL_PROMPT=0` so git fails
fast instead of opening `/dev/tty` to prompt for credentials — prompts would
hang or interleave during parallel sync. `GIT_ASKPASS` is left alone so
non-interactive credential helpers keep working. Only ergo-initiated git
commands go through `ExecRunner`; user commands from `ergo run` use
`workspace.runInDir` and are unaffected.

When a clone/pull error looks auth-related (`authHint` matches strings like
"terminal prompts disabled" or "Permission denied (publickey"), `Clone` and
`Pull` append a one-line hint pointing at the `[git].protocol` setting.

`RewriteToSSH(url)` in `url.go` converts plain-host http(s) URLs to scp form
(`git@host:owner/repo.git`); everything else (ports, embedded credentials,
non-http schemes, local paths) passes through unchanged. It is called from
`workspace.syncRepo` when `SyncOptions.RewriteURLsToSSH` is set (from
`[git].protocol = "ssh"`).

Public functions:

| Function                      | Underlying command                                                                             |
| ----------------------------- | ---------------------------------------------------------------------------------------------- |
| `CheckPath()`                 | `exec.LookPath("git")`                                                                         |
| `Clone(r, url, dest, branch)` | `git clone [--branch <b>] <url> <dest>` (retries without `--branch` if the remote lacks it)    |
| `Pull(r, dir)`                | `git pull --ff-only` (refuses non-FF merges; returns `ErrEmptyRemote` if upstream ref missing) |
| `Init(r, dir)`                | `git init`                                                                                     |
| `Status(r, dir)`              | `git status --porcelain` → bool dirty                                                          |
| `BehindCount(r, dir)`         | `git rev-list --count HEAD..@{u}` → int                                                        |
| `CurrentBranch(r, dir)`       | `git rev-parse --abbrev-ref HEAD`                                                              |
| `RepoRoot(r, dir)`            | `git rev-parse --show-toplevel` (used by detection)                                            |

`Pull` uses `--ff-only` deliberately — sync should never create merge commits.

`Clone` has a single fallback: when called with a non-empty `branch` and git
fails with "Remote branch ... not found in upstream origin" (typical for a
freshly-created empty repo, or when the TOML pins a branch that doesn't exist
on the remote), `Clone` removes the partial destination directory and retries
without `--branch`. The retry uses the remote's default branch, or — for an
empty repo — produces a working tree with no checkout, ready for a first push.
Any other clone failure (auth, repo not found, network) propagates immediately.

`Pull` has a complementary fallback: when git reports "no such ref was
fetched" or "couldn't find remote ref refs/heads/…" — the same empty-repo
scenario after `Clone` has already returned — it returns the sentinel
`ErrEmptyRemote` (wrapped). `workspace.syncRepo` checks for it via
`errors.Is` and reports the repo as `skipped` instead of `failed`, so an
empty repo round-trips through `ergo sync` cleanly until the first push.

---

## `internal/github`

File: [`github.go`](../../ergo/internal/github/github.go)

Mirror of `internal/git` but for `gh`. Used **only** by `ergo update`.

- `ergoRepo = "juan7732/ergo"` is hardcoded — explicit per the tenet
  "don't add configuration for things that should be hardcoded".
- `LatestRelease(r)` runs `gh release list --repo ... --limit 1 --json tagName --jq '.[0].tagName'`.
- `DownloadRelease(r, tag, pattern, destDir)` runs
  `gh release download <tag> --repo ... --pattern <pattern> --dir <destDir>`.

The `ExecRunner` here uses `cmd.CombinedOutput()` (vs. separated streams in
`git`) because `gh` writes informational output to stderr that's useful in
error messages.

---

## `internal/vscode`

Files:
- [`generate.go`](../../ergo/internal/vscode/generate.go) — `Generate`, `Filter`, `ergoMeta`, `wsFolder`, `codeWorkspace`.
- [`diff.go`](../../ergo/internal/vscode/diff.go) — `WriteIfChanged`.
- Tests: [`generate_test.go`](../../ergo/internal/vscode/generate_test.go),
  [`diff_test.go`](../../ergo/internal/vscode/diff_test.go).

`Generate(cfg, filter)` produces canonical bytes:

1. Top-level `"ergo"` JSON object: `{"workspace-name": "...", "filter": {...}?}`.
2. `"folders"` array, in this exact order:
   - `{"name": "root", "path": "."}` — non-negotiable, hardcoded first entry.
   - Every `[[repos]]` in TOML order, with `name = path = EffectiveName()` and
     optional folder-level `settings` from `repo.vscode_settings`.
   - Every `[[folders]]` in TOML order, same shape.
3. Optional top-level `"settings"` from `[workspace.vscode.settings]`.

Trailing newline appended for POSIX-friendliness and clean diffs.

`Filter` is a struct with `omitempty` JSON tags so absent fields don't appear
in the output — clearing the filter means passing `nil`.

`WriteIfChanged(path, content)`:

- Reads existing bytes; returns `(false, nil)` if equal to `content`.
- Otherwise creates parent dirs (mode `0700`) and writes the file (mode `0600`).
- Returns `(true, nil)` on successful write.

---

## `internal/workspace`

The orchestration core. Each file owns one concern.

### `detect.go`

`Detect(cwd, runner) Detection`

Three strategies (first-match-wins):

1. **Walk-up `.code-workspace` search.** From `cwd` upward, for each directory
   read its entries, and for each `*.code-workspace` file try to JSON-parse it
   into `ergoCodeWorkspace`. If it has an `ergo.workspace-name`, return it.
   Malformed JSON is treated as "not ours" and skipped silently.
2. **Match against known workspace_root.** If `cwd` starts with
   `<workspace_root>/`, take the first path segment as a candidate name and
   confirm it exists in `~/.ergo/workspaces/`.
3. **Standalone repo.** `git rev-parse --show-toplevel`. If it succeeds,
   set `IsStandaloneRepo=true` and `StandaloneRepoRoot=<root>`.

Returns a zero `Detection{}` (all empty) when none match.

### `resolve.go`

`Resolve(nameArg, cwd, runner) ResolveResult`

Implements the six-step rule documented in [03-commands.md](03-commands.md#workspace-resolution).

- `matchNames` first checks if the pattern contains glob metachars (`*?[`).
  If so, compile with `gobwas/glob` and match case-insensitively. Otherwise
  case-insensitive substring match.
- `closestName` returns the workspace name with the longest common case-folded
  byte prefix — used to suggest "did you mean…?" in error messages.
- A single partial match resolves directly (no TUI). Multiple matches return
  `Candidates` for the TUI selector.

### `filter.go`

`ApplyRepoFilter(repos, opts) []config.Repo`

Pure function — no I/O, no globals. Compiles `--name` glob once at the top.
See [03-commands.md](03-commands.md#filter-flags) for the precedence rules.

### `manager.go`

The heart of `ergo sync` and `ergo open`'s materialization.

`Sync(cfg, opts, runner) (SyncResult, error)`

1. Ensure `WorkspaceDir` exists (`MkdirAll` mode `0700`).
2. Build the `knownNames` set from repos+folders for orphan detection.
3. `syncRepos` — sequential or bounded-parallel via errgroup+semaphore.
   Per-repo errors are stored in `RepoResult.Err`; goroutines never return errors.
4. `syncFolder` for each folder — always sequential (no network I/O so the
   parallel speedup wouldn't justify the complexity).
5. `findOrphans` — `os.ReadDir(WorkspaceDir)` filtered against `knownNames`.
   Errors here are non-fatal (printed to stderr).

Per-repo decision tree (`syncRepo`):

```
                           ┌─── exists? ───┐
                           │               │
                           No              Yes
                           │               │
                          Clone            ▼
                       (with --branch     AutoPull?
                       if specified)      ├── No  → skipped
                                          └── Yes → pull --ff-only
```

Per-folder (`syncFolder`):

```
MkdirAll → if Git=true and no .git/ → git init
```

Public helper: `DeleteOrphan(workspaceDir, name)` — `os.RemoveAll`, used by
`sync --force`.

### `runner.go`

`RunAcrossTargets(targets, opts) ([]RunResult, error)`

Cross-directory command executor. Sequential or bounded-parallel.
`runInDir` captures combined stdout+stderr in one buffer (matching how a TTY
user would see interleaved output), records `ExitCode` (0 on success, the
ExitError code otherwise), and reserves `Err` for **infrastructure** failures
(binary not on PATH, missing dir, etc.).

Fail-fast behavior in parallel: the goroutine that sees a failure flips a
shared `halted` bool under mutex and `cancel()`s the context. Other goroutines
check `halted` after acquiring the semaphore and skip if set. The result slice
is sized up-front so completed targets retain their position; the final
collection step filters empty entries (those that never ran).

### `status.go`

`GatherStatus(cfg, wsDir, runner, parallel, batchSize)` — bounded-parallel
gather of `RepoStatusEntry` per repo. Git errors are absorbed into best-effort
defaults (e.g. missing upstream → `Behind=0`). `os.IsNotExist` on the repo dir
sets `Uncloned=true`.

`GatherSingleRepoStatus(dir, name, group, runner)` — used by `status` in
standalone-repo mode.

### `state.go`

JSON-backed performance cache. Best-effort: any read error returns a zero
struct without surfacing the error (callers can't distinguish "first run" from
"cache deleted" from "JSON corrupt"). `SaveState` does propagate errors.

---

## `internal/tui`

| File                  | Component                                  | Used by                               |
| --------------------- | ------------------------------------------ | ------------------------------------- |
| `app.go`              | `Run` / `RunInline` helpers                | every cmd that launches a TUI         |
| `styles.go`           | Lipgloss palette + `KeybindingHint` helper | all TUI components                    |
| `init_wizard.go`      | `InitWizard` model                         | `cmd/init.go`                         |
| `add_form.go`         | `AddForm` model                            | `cmd/add.go` (TUI default)            |
| `remove_select.go`    | `RemoveSelect` model                       | `cmd/remove.go` (TUI default)         |
| `group_select.go`     | `GroupSelect` model                        | `cmd/show.go` (no-arg TUI)            |
| `workspace_select.go` | `WorkspaceSelect` model                    | `resolveWorkspaceName` when ambiguous |
| `repo_table.go`       | `RenderRepoTable`, `ShortRepoLine`         | `cmd/status.go`, `cmd/list.go`        |
| `run_output.go`       | `PrintRunResult`                           | `cmd/run.go`                          |

See [05-tui.md](05-tui.md) for details on each model.

---

## `cmd/`

Beyond what's documented in [03-commands.md](03-commands.md), the shared helpers
in [`helpers.go`](../../ergo/cmd/helpers.go) deserve mention:

- `isTerminal()` — checks `os.ModeCharDevice` on stdin to decide whether to
  prompt interactively.
- `currentDir()` — `os.Getwd` with wrapped error.
- `workspaceDir(globalCfg, name)` — `<workspace_root>/<name>` with `~` expanded.
- `execRunner()` — returns the production `git.ExecRunner{}` (centralized so
  tests in `cmd/` could swap it).
- `filterOptsFromFlags(cmd, excludedGroups)` — collects `--name`, `--group`,
  `--tags` into a `workspace.FilterOptions`.

`resolveWorkspaceName` in [`validate.go`](../../ergo/cmd/validate.go) (yes,
co-located by accident — it's used from many commands) wraps `workspace.Resolve`
and falls back to the `WorkspaceSelect` TUI when the result is a candidate list.
