# Architecture

## Layered Dependency Graph

```
                    ┌──────────────────────────┐
                    │          main.go         │
                    │  (sets version, Execute) │
                    └─────────────┬────────────┘
                                  │
                    ┌─────────────▼────────────┐
                    │            cmd/          │  ← Cobra commands (UI layer)
                    │  root, init, open, sync, │
                    │  add, remove, edit,      │
                    │  status, list, run,      │
                    │  show, validate, update  │
                    └────┬────────┬──────┬─────┘
                         │        │      │
              ┌──────────▼─┐    ┌─▼────┐ │
              │ internal/  │    │ tui/ │ │
              │  workspace │    │      │ │
              └──┬───────┬─┘    └──────┘ │
                 │       │               │
        ┌────────▼┐  ┌───▼────┐    ┌─────▼────┐
        │ config  │  │  git   │    │  github  │
        └─────────┘  └────────┘    └──────────┘
                 │
        ┌────────▼─────────┐
        │   vscode/        │
        │ (generate+diff)  │
        └──────────────────┘
```

Rules of the graph (enforced informally):

- **`cmd/`** is the only package that knows about Cobra and stdout formatting.
  Each command is a thin orchestrator: parse args/flags, load config, call into
  `internal/*`, render result.
- **`internal/workspace`** is the orchestration layer for the actual work
  (sync, status, run, detect, resolve, filter, state).
- **`internal/config`** has no dependencies on other internal packages.
- **`internal/git`** and **`internal/github`** are thin adapters over external
  CLIs (`git`, `gh`). Each exposes a tiny `Runner` interface to enable testing
  without spawning real processes.
- **`internal/vscode`** is pure: it serializes a `WorkspaceConfig` into
  `.code-workspace` JSON and compares bytes for smart writes.
- **`internal/tui`** owns all Bubble Tea models, styles, and rendering helpers.
  It is imported by `cmd/` for interactive flows.

## Key Types and Where They Live

### Configuration (`internal/config`)

| Type                                 | File          | Notes                                                              |
| ------------------------------------ | ------------- | ------------------------------------------------------------------ |
| `GlobalConfig`                       | `types.go`    | Parsed `~/.ergo/config.toml`                                       |
| `DefaultsConfig`                     | `types.go`    | `workspace_root`, `default_branch`                                 |
| `ParallelConfig`                     | `types.go`    | `enabled`, `batch_size`                                            |
| `SyncConfig`                         | `types.go`    | `auto_pull`                                                        |
| `RunConfig`                          | `types.go`    | `excluded_groups []string`                                         |
| `WorkspaceConfig`                    | `types.go`    | Parsed `~/.ergo/workspaces/<name>.toml`                            |
| `WorkspaceMeta`                      | `types.go`    | `name` + nested `vscode.settings`                                  |
| `Repo`                               | `types.go`    | `URL`, `*Name`, `*Branch`, `Tags`, `Group`, per-folder VS settings |
| `Folder`                             | `types.go`    | `Name`, `Git bool`, per-folder VS settings                         |
| `ValidationError`/`ValidationErrors` | `validate.go` | Returned by `Validate(WorkspaceConfig)`                            |

Notable design choices:

- `Repo.Name` and `Repo.Branch` are pointers so we can distinguish "unset" (nil)
  from "explicitly empty". Unset → derive from URL / fall back to default.
- `Repo.EffectiveName()` collapses the pointer logic into one call site used
  everywhere downstream.

### Workspace orchestration (`internal/workspace`)

| Type                                                         | File         | Purpose                                                            |
| ------------------------------------------------------------ | ------------ | ------------------------------------------------------------------ |
| `Detection`                                                  | `detect.go`  | Output of `Detect(cwd)` — workspace name or standalone-repo info   |
| `ResolveResult`                                              | `resolve.go` | Output of `Resolve()` — `Name` (unambiguous) or `Candidates` (TUI) |
| `FilterOptions`                                              | `filter.go`  | All filter flag inputs + excluded-groups config                    |
| `RepoAction` / `FolderAction`                                | `manager.go` | Enums of what `Sync` did with each entry                           |
| `SyncOptions` / `SyncResult` / `RepoResult` / `FolderResult` | `manager.go` | Inputs and outputs of `Sync`                                       |
| `RepoStatusEntry`                                            | `status.go`  | Per-repo state (branch, dirty, behind, uncloned, group)            |
| `RunTarget`                                                  | `runner.go`  | Named directory the command runs in                                |
| `RunResult` / `RunOptions`                                   | `runner.go`  | I/O for `RunAcrossTargets`                                         |
| `WorkspaceState` / `RepoStateEntry`                          | `state.go`   | JSON-cached metadata in `~/.ergo/state/<ws>.json`                  |

### Git / GitHub adapters

Both `internal/git/git.go` and `internal/github/github.go` define an identical
`Runner` interface:

```go
type Runner interface {
    Run(dir, name string, args ...string) (string, error)
}
```

…and an `ExecRunner` implementation that shells out via `os/exec`. This is the
**only** preemptive interface in the codebase (per the tenet "earn every
abstraction"); it exists to make package-level tests possible without spawning
real `git` or `gh` processes.

### VS Code (`internal/vscode`)

| Type            | File          | Purpose                                                 |
| --------------- | ------------- | ------------------------------------------------------- |
| `Filter`        | `generate.go` | Optional filter recorded under `ergo.filter`            |
| `ergoMeta`      | `generate.go` | The `ergo` JSON object: `workspace-name` + filter       |
| `wsFolder`      | `generate.go` | Single folder entry in `folders[]`                      |
| `codeWorkspace` | `generate.go` | Top-level JSON shape with `ergo`, `folders`, `settings` |

Public functions:

- `Generate(cfg WorkspaceConfig, filter *Filter) ([]byte, error)` — produces
  the canonical `.code-workspace` bytes (root folder injected first,
  trailing newline appended).
- `WriteIfChanged(path string, content []byte) (bool, error)` — no-op when
  bytes already match (smart regeneration).

## Conventions Enforced Throughout

- **Error wrapping.** Every error returned from internal functions is wrapped
  with `fmt.Errorf("doing X: %w", err)`. User-facing messages are lowercase
  with no trailing punctuation (e.g. `"git not found on PATH: install git and retry"`).
- **Functions accept interfaces, return structs.** `Runner` is the only
  injected interface; everything else returns concrete types.
- **No `utils` / `helpers` package.** Cross-cutting helpers live in
  [`cmd/helpers.go`](../../ergo/cmd/helpers.go) (`isTerminal`, `currentDir`,
  `workspaceDir`, `execRunner`, `filterOptsFromFlags`).
- **Testing** is table-driven, `TestFunctionName_Scenario`, lives next to the
  code under test. Test files exist for `config`, `git`, `vscode`, and the
  `workspace` package.
- **Concurrency** uses `errgroup.WithContext` plus a buffered channel as a
  semaphore for bounded fan-out. Per-task errors are stored back into a result
  slice (with `sync.Mutex`) — `errgroup.Wait()` only surfaces infrastructure
  errors. This is the pattern in `Sync`, `GatherStatus`, and `RunAcrossTargets`.

## Why This Layout

- One file per cobra command makes adding/removing commands a localized change.
- Putting orchestration in `internal/workspace` keeps `cmd/` files small (most
  are <250 lines, almost all of which is wiring + output formatting).
- The `internal/` boundary prevents external Go consumers from depending on
  ergo internals — only `main.go` and the CLI itself are public surface.
- TUI components are isolated so non-TUI callers (scripts, `--short` modes)
  never accidentally pull in Bubble Tea startup cost.
