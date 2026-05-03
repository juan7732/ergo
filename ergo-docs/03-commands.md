# Command Reference

Every command is implemented in a single file under [`cmd/`](../../ergo/cmd/).
Every cross-repo command supports filter flags (see "Filter flags" at the bottom).

---

## `ergo init [workspace-name]`
File: [`cmd/init.go`](../../ergo/cmd/init.go)

Create a new workspace TOML via guided TUI (Bubble Tea wizard).

- Prefills the workspace name with the optional positional arg.
- Loops on repo URL → optional `name`/`branch`/`tags`/`group` → blank URL ends.
- Loops on folder name → `git init?` → blank name ends.
- Final confirmation summary.
- Writes `~/.ergo/workspaces/<name>.toml`. **Does not create any directories
  on disk and does not clone anything** — that is `ergo open`'s job.
- On Esc/Ctrl-C the wizard cancels and the file is not written ("cancelled").

---

## `ergo open [workspace-name]`
File: [`cmd/open.go`](../../ergo/cmd/open.go)

Open the workspace in VS Code, materializing it on first use.

Decision tree (`runOpen`):

1. Resolve workspace name (see resolution rules in [03-commands.md](#workspace-resolution)).
2. Load global config and workspace TOML.
3. Compute expected `<wsDir>/<name>.code-workspace` bytes via `vscode.Generate`.
4. **Fast path** — if `wsDir` exists *and* the existing `.code-workspace`
   bytes match the expected output exactly, skip everything and `exec code <path>`.
5. Otherwise:
   - If `wsDir` is missing → first-time materialization: clone all repos
     (with `AutoPull = false`), create folders, run `git init` where requested.
     Persist state cache.
   - Regenerate `.code-workspace` (smart-write — writes only if changed).
   - `exec code <path>`.

Notes:

- When `wsDir` exists but the TOML changed since last sync, `open` regenerates
  the `.code-workspace` but does **not** re-clone — that is `sync`'s job
  (`// REVIEW:` comment in source acknowledges spec is silent on this case).
- Aborts with a friendly message if `code` is not on `$PATH`.

---

## `ergo sync [workspace-name]`
File: [`cmd/sync.go`](../../ergo/cmd/sync.go)

Reconcile the on-disk workspace with its TOML.

Behavior per repo:

| State on disk | `auto_pull` | Action                                     |
| ------------- | ----------- | ------------------------------------------ |
| missing       | n/a         | `git clone` (with `--branch` if specified) |
| present       | `true`      | `git pull --ff-only`                       |
| present       | `false`     | skipped                                    |

Behavior per folder:

| State on disk       | `git` flag | Action                      |
| ------------------- | ---------- | --------------------------- |
| missing             | n/a        | `os.MkdirAll` (mode `0700`) |
| present (no .git)   | `true`     | `git init`                  |
| present (with .git) | `true`     | skipped                     |

After repo/folder reconciliation, regenerates `.code-workspace` (smart-write)
and reports orphans (directories on disk not in the TOML).

**Sync never deletes by default.** Orphans are listed; `--force` deletes them
after a y/N confirmation prompt on stdin.

Flags:

| Flag             | Purpose                                                                                                      |
| ---------------- | ------------------------------------------------------------------------------------------------------------ |
| `--force`        | Delete orphan directories after confirmation.                                                                |
| `--add`          | Reverse-sync: prompt to add orphan directories to the TOML (repos detected via `git remote get-url origin`). |
| `--name <glob>`  | Filter (see filter flags)                                                                                    |
| `--group <name>` | Filter                                                                                                       |
| `--tags t1,t2`   | Filter (any-match)                                                                                           |

Exit code is non-zero (with summary error) if any per-repo or per-folder
operation failed.

---

## `ergo status [workspace-name]`
File: [`cmd/status.go`](../../ergo/cmd/status.go)

Per-repo state table: branch, dirty, behind, group.

- Inside a workspace → status for all (filtered) repos.
- Outside a workspace but inside a git repo → status for that single repo
  (uses `workspace.Detect` + `GatherSingleRepoStatus`).
- `--short` / `-s` switches to one-line-per-repo for scripting.

Status values shown:

- `clean` — no uncommitted changes.
- `dirty` — uncommitted changes present (`git status --porcelain`).
- `uncloned` — directory not on disk yet.

`Behind` column shows commits behind upstream from `git rev-list --count HEAD..@{u}`;
displays `—` when zero or no upstream.

Filter flags: `--name`, `--group`, `--tags`.

---

## `ergo add [workspace-name]`
File: [`cmd/add.go`](../../ergo/cmd/add.go)

Add a repo or folder to a workspace TOML.

Three invocation modes:

1. `ergo add` (no subcommand) — TUI form picks repo vs folder, collects fields,
   warns on collision, writes TOML, then prompts to sync (if stdin is a TTY).
2. `ergo add repo <url> [--name=...] [--tags=t1,t2] [--group=...]` —
   non-interactive shorthand.
3. `ergo add folder <name> [--git]` — non-interactive shorthand.

Collision check is `checkNameCollision()` — name must not match any existing
repo's effective name or folder name.

After writing, calls `promptSync()` which reads y/N from stdin only if
`isTerminal()` returns true.

---

## `ergo remove [workspace-name]`
File: [`cmd/remove.go`](../../ergo/cmd/remove.go)

Three modes mirroring `add`:

1. `ergo remove` — multi-select TUI picker (`tui.NewRemoveSelect`).
2. `ergo remove repo <name>` — by name.
3. `ergo remove folder <name>` — by name.

By default only the TOML entry is removed. With `--force`, the matching
directory is deleted from disk after a confirmation prompt that lists every
path that would be removed.

---

## `ergo edit [workspace-name]`
File: [`cmd/edit.go`](../../ergo/cmd/edit.go)

Resolves the workspace and `exec`s `code ~/.ergo/workspaces/<name>.toml`.
Errors if `code` isn't on `$PATH`.

---

## `ergo list`
File: [`cmd/list.go`](../../ergo/cmd/list.go)

Prints a Unicode-bordered table:

```
┌───────────────┬───────┬────────────┐
│ Workspace     │ Repos │ Status     │
├───────────────┼───────┼────────────┤
│ ml-projects   │ 4     │ synced     │
│ side-projects │ 8     │ not synced │
└───────────────┴───────┴────────────┘
```

`status`:

- `synced` — directory exists at `<workspace_root>/<name>/`.
- `not synced` — TOML defined but workspace not materialized.

A `// REVIEW:` comment notes that ANSI escape bytes inflate `%-*s` padding for
the status column when colors are enabled — visible misalignment is a known
follow-up.

---

## `ergo show [group | all]`
File: [`cmd/show.go`](../../ergo/cmd/show.go)

Filter the VS Code view by regenerating `.code-workspace` to include only
matching repos. Records the active filter in the `ergo` JSON object.

| Invocation                         | Effect                                   |
| ---------------------------------- | ---------------------------------------- |
| `ergo show <group>`                | filter to repos with `group = "<group>"` |
| `ergo show --tag=<t>` (repeatable) | filter to repos tagged `<t>` (any-match) |
| `ergo show all`                    | clear the active filter                  |
| `ergo show` (no arg, no flag)      | TUI multi-select of groups+tags          |

Constraints:

- Must be run from inside an ergo workspace (uses `workspace.Detect` on CWD,
  not `Resolve`). Errors otherwise.
- Workspace must already be materialized (`.code-workspace` must exist).
- Only `.code-workspace` is modified. Never the TOML, never the filesystem.
- Always includes the `root` folder + all `[[folders]]` regardless of filter.
  (Filter applies to `[[repos]]` only.)

---

## `ergo run [workspace-name] -- <command> [args...]`
File: [`cmd/run.go`](../../ergo/cmd/run.go)

Execute an arbitrary command in each repo's directory.

Behavior:

- `--` is required to separate ergo flags from the command. `cmd.ArgsLenAtDash()`
  determines the split point.
- **Standalone fallback:** if no workspace name and CWD is a git repo, runs the
  command in that single repo. Filter flags and `excluded_groups` are ignored
  in this mode.
- Inside a workspace:
  1. Apply filter flags (`--name`, `--group`, `--tags`).
  2. Honor `[run].excluded_groups` only if no explicit filter and `--all` is
     not set. `--include-group=<g>` is an exception that re-includes one
     specific excluded group.
  3. Optionally include `[[folders]]` via `--include-folders`.
  4. Run the command in each target directory (parallel up to `batch_size`).
- Output is rendered by `tui.PrintRunResult` per repo as it finishes.
  Header format: `━━━ <repo-name> ━━━`.

Flags:

| Flag                  | Purpose                                                                 |
| --------------------- | ----------------------------------------------------------------------- |
| `--fail-fast`         | Cancel remaining work after first non-zero exit / infrastructure error. |
| `--include-folders`   | Also run in `[[folders]]`.                                              |
| `--include-group=<g>` | Re-include a specific excluded group.                                   |
| `--all`               | Ignore `excluded_groups` entirely.                                      |
| `--name <glob>`       | Filter                                                                  |
| `--group <name>`      | Filter                                                                  |
| `--tags t1,t2`        | Filter                                                                  |

Exit: ergo exits non-zero with `"<n> repo(s) failed"` if any target had a
non-zero exit code or infrastructure error.

---

## `ergo validate [workspace-name]`
File: [`cmd/validate.go`](../../ergo/cmd/validate.go)

Run the validation rules listed in [02-configuration.md](02-configuration.md#collision-rules).

- Per-issue output: `• repos[2]: derived name "utils" collides with repos[0]`.
- `--all` validates every workspace under `~/.ergo/workspaces/`.
- Returns non-zero on any failure.

---

## `ergo update`
File: [`cmd/update.go`](../../ergo/cmd/update.go)

Self-update by downloading the latest GitHub release of the hardcoded repo
`juan7732/ergo` (constant in [`internal/github/github.go`](../../ergo/internal/github/github.go)).

Steps:

1. Verify `gh` is on `$PATH`.
2. `gh release list --repo juan7732/ergo --limit 1 --json tagName --jq '.[0].tagName'`.
3. Compare tags (strip leading `v`). If equal and current is not `dev`,
   exit with "already up to date".
4. Resolve own executable path (and `EvalSymlinks`) so we replace the real binary.
5. `gh release download <tag> --repo juan7732/ergo --pattern ergo-darwin-arm64 --dir <exeDir>`.
6. `chmod 0755`, then `os.Rename` (atomic on the same filesystem) over the
   running binary.
7. On any failure after download starts, the deferred cleanup removes the
   downloaded artifact.

`dev` builds always attempt to update.

---

## `ergo --version`

Cobra's built-in version flag. Set in `cmd.Execute(v)`. The version string is
embedded at build time via `-ldflags "-X main.version=..."` (defaults to `"dev"`).

---

## Filter flags

Available on `sync`, `status`, `run`, and (via `--tag`) `show`. Implemented in
[`internal/workspace/filter.go`](../../ergo/internal/workspace/filter.go).

| Flag               | Behavior                                                                                                         |
| ------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `--name <glob>`    | Glob against repo's effective name, case-insensitive. Falls back to substring match if pattern fails to compile. |
| `--group <name>`   | Exact match against `repo.group`.                                                                                |
| `--tags t1,t2,...` | Repo passes if **any** of its tags is in the list.                                                               |

`ApplyRepoFilter` rules:

1. If any of `--name`, `--group`, or `--tags` is set, `excluded_groups` is **not**
   applied (an explicit filter overrides exclusion).
2. If none are set and `--all` is not set, repos in `[run].excluded_groups` are
   skipped — except those whose group equals `--include-group`.
3. All filter flags AND together (a repo must pass every active filter).

---

## Workspace resolution

`workspace.Resolve(nameArg, cwd, runner)` ([`resolve.go`](../../ergo/internal/workspace/resolve.go))
implements the six-step rule:

1. `nameArg` exact match → use it.
2. `nameArg` partial/glob match → single match resolves directly; multiple
   matches return `Candidates` (TUI selector).
3. `nameArg` provided, no match → error including a suggestion based on longest
   common prefix (`closestName`).
4. `nameArg == "."` → use CWD-detected workspace, error if not in one.
5. `nameArg == ""`, CWD inside a workspace → use detected workspace.
6. `nameArg == ""`, CWD outside any workspace → return all workspaces as `Candidates`.

`Detect` itself ([`detect.go`](../../ergo/internal/workspace/detect.go)) tries
three strategies in order: walk up looking for a `.code-workspace` containing an
`ergo` JSON key; match CWD against `<workspace_root>/<name>/...`; fall back to
`git rev-parse --show-toplevel` for standalone-repo detection.
