# Operational Semantics

Cross-cutting behaviors that hold across every command.

## Idempotency

Per the tenet "Safe by default, destructive by intent":

- `ergo sync` running twice in a row produces no second-run side effects:
  every repo reports `pulled` (or `skipped` if `auto_pull=false`), no clones
  happen, the `.code-workspace` file is not rewritten (smart regen → bytes
  unchanged → `WriteIfChanged` returns `(false, nil)`).
- `ergo open` is the same: fast-path skips `Sync` entirely when the dir exists
  and the workspace file is current; smart regen handles the slow path.
- `ergo show <group>` against an already-set filter prints
  "filter already set to group X".
- `ergo show all` against an unfiltered workspace prints "no filter was active".
- `ergo init <name>` against an existing workspace will overwrite the TOML
  on confirm; the wizard does not warn (potential follow-up).

## Smart regeneration of `.code-workspace`

Sequence:

1. `vscode.Generate(cfg, filter)` — pure, deterministic, returns canonical bytes
   (with trailing newline).
2. `vscode.WriteIfChanged(path, content)` — reads existing bytes, byte-equality
   check, only writes if different.

This avoids file-change events in VS Code that would trigger restarts of
language servers, file watchers, etc. It also makes the workspace file
diff-friendly under version control.

## Show-filter preservation

An active `ergo show` filter is recorded in the `.code-workspace` under
`ergo.filter`. Any command that regenerates the file **preserves** it:

1. `vscode.ReadFilter(path)` reads the recorded filter back (tolerating
   unknown fields).
2. The folders list is filtered through `workspace.ApplyRepoFilter` and the
   filter is passed back into `vscode.Generate`, so it stays recorded.
3. `open`'s fast-path currency check compares against the *filtered* expected
   bytes, so a filtered file is not treated as stale.

Semantics:

- **The filter is purely a view concern.** `sync` still operates on the full
  TOML — repos hidden by the filter are still cloned and pulled. The
  operation set is governed only by the explicit `--name`/`--group`/`--tags`
  flags. (Destruction scope, likewise, is always computed from the full
  config.)
- **Surfaced, never silent.** When a preserved filter is active, `sync` and
  `open` print one note line, and table-format `status` shows it as a header:

  ```
  note: show filter active (group "ml") — 3 of 12 repos visible; run 'ergo show all' to clear
  ```

  The note goes to stderr under `open --print-dir` (stdout stays clean for
  shell capture). It is **not** added to `--json` output — JSON consumers
  read the filter from `ergo show --json`.
- **Fail-open.** A malformed or unreadable workspace file degrades to the
  pre-preservation behavior: regenerate the full, unfiltered view. Filter
  recovery never fails a sync or open.
- A filter that no longer matches any repo is still preserved (the note line
  reads `0 of N repos visible`); clearing it is the user's call via
  `ergo show all`.

## Never deletes by default

| Command           | Default behavior         | Destructive variant                      |
| ----------------- | ------------------------ | ---------------------------------------- |
| `ergo sync`       | warns about orphans      | `--force` deletes after y/N confirmation |
| `ergo remove ...` | removes from TOML only   | `--force` deletes from disk after y/N    |
| `ergo update`     | atomic replace of binary | (always replaces, but cleanup-on-fail)   |

`os.Rename` in `update` is atomic on the same filesystem, so a failed update
cannot leave a half-written binary in place.

## Parallelism model

Implemented in three places using the same pattern:

- `internal/workspace/manager.go::syncRepos`
- `internal/workspace/status.go::GatherStatus`
- `internal/workspace/runner.go::RunAcrossTargets`

Pattern:

1. Pre-allocate a `[]Result` with `len(items)` so each goroutine writes to its
   own index — no append, no reordering.
2. Buffered channel of `struct{}` sized `BatchSize` acts as a semaphore.
3. `errgroup.WithContext` for goroutine lifecycle. Per-task errors live in
   the result struct, not in the error returned from the goroutine; the
   group's `Wait()` only surfaces infrastructure errors.
4. A `sync.Mutex` protects writes to the result slice (and any shared state
   like `halted` in fail-fast).
5. Optional `Progress` / `OnResult` callback fires after each item completes.
   Callbacks must be thread-safe (they are, in current callers — they only
   write to stdout via `fmt.Fprintf`, which is goroutine-safe per line).

`BatchSize <= 0` is clamped to `1`. `Parallel = false` means strictly sequential.

## Confirmation prompts

A few commands prompt on stdin:

- `sync --force` — y/N to delete orphans.
- `sync --add` — y/N to add orphans to TOML.
- `add` (TUI flow only) — y/N to sync now (after writing TOML).
- `remove --force` — y/N to delete files.

All use `bufio.Scanner` or `bufio.Reader.ReadString('\n')` and accept lowercase
`"y"` only. Any other input cancels.

`promptSync` skips the prompt entirely when `!isTerminal()` — important for
scripts piping into ergo.

## Error model

Per the project Go conventions:

- All errors wrap with context: `fmt.Errorf("doing X: %w", err)`.
- User-facing messages are lowercase, no trailing punctuation, actionable.
- Sentinel errors are not currently used; `errors.As` is used to unwrap
  `config.ValidationErrors`.
- Cobra's `RunE` returns drive the process exit code.

### Notable error categories

| Failure                                     | Behavior                                                                                                |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `git`/`gh`/`code` missing on PATH           | Error with install hint, immediate non-zero exit.                                                       |
| Clone fails for one repo                    | Recorded in `RepoResult.Err`, sync continues, summary error at end.                                     |
| Clone with `--branch` on empty remote       | `git.Clone` removes the partial dest and retries without `--branch`; reported as `cloned` on success.   |
| Pull against empty remote (no upstream ref) | `git.Pull` returns `ErrEmptyRemote`; `syncRepo` reports `skipped` instead of `failed`.                  |
| TOML parse error                            | Wrapped with file path, command exits.                                                                  |
| Workspace TOML missing                      | `LoadWorkspace` returns wrapped `os` error.                                                             |
| Validation error                            | `ValidationErrors` printed line by line.                                                                |
| `ergo run -- <cmd>` — non-zero exit         | Recorded in `RunResult.ExitCode`, run continues unless `--fail-fast`, summary error sets non-zero exit. |
| `ergo run -- <cmd>` — binary missing        | `RunResult.Err` (infrastructure failure).                                                               |
| Update with no new release                  | Friendly message "ergo is already up to date".                                                          |

## Exit codes

ergo relies on Cobra's default exit handling:

- `0` on success (no `RunE` error returned).
- `1` on any error returned from `RunE` (Cobra's default).

There is no custom exit-code scheme. `--fail-fast` and the `run` summary
produce a non-zero exit by returning a wrapped error from `RunE`.

## Logging / output streams

- All user-visible output goes to **stdout** (`cmd.OutOrStdout()`).
- Warnings (state save failures, orphan-scan errors) go to **stderr**.
- Under `--json` (status/list/validate/show), stdout carries exactly one JSON
  document and nothing else; warnings stay plain text on stderr. See the
  [JSON output contract](03-commands.md#json-output-contract).
- No structured logging library — `fmt.Fprintf` everywhere.
- Color is enabled by default; the persistent `--no-color` flag is registered
  on the root command but currently unwired (`// REVIEW`-worthy).
- Test harness sets `NO_COLOR=1` and `TERM=dumb` for stable string assertions.

## Caching

Only one cache: `~/.ergo/state/<workspace>.json` (see [02-configuration.md](02-configuration.md)).
It records `last_sync` and per-repo `last_pulled`. `commit_hash` is reserved
but unused. Missing or corrupt state is treated as "first run" — never an
error path.

`workspace.LoadState` deliberately swallows file-read and JSON errors; only
`SaveState` propagates them, and even then callers print a warning rather
than failing the whole command (see `cmd/sync.go` and `cmd/open.go`).

## Concurrency hazards consciously avoided

- No global mutable state. Configuration is loaded fresh per command.
- The git `Runner` is stateless. The same `ExecRunner{}` value is shared
  across goroutines safely.
- TUI models are run on the main goroutine; Bubble Tea owns its own scheduling.
