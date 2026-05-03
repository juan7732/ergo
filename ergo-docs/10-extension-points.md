# Extension Points

What's deliberately deferred and where it would slot in. Sourced from
[`token-maximizer/ergo-spec.md`](../../token-maximizer/ergo-spec.md) §10
and [`scratch/ergo-implementation-plan.md`](../ergo-implementation-plan.md).

## v2 candidates

### Doc cache

**Goal:** mount per-repo documentation snapshots into the workspace as a
read-only folder so Copilot can ground answers in upstream docs without
needing live internet access.

**Slot:** `[[repos]].docs` table inside the workspace TOML, plus a derived
folder added to `.code-workspace` under a special name (e.g. `docs/`).

**Implementation hooks already in place:**

- `vscode.Generate` already produces folder entries; an extra section to
  generate doc folders is a 10-line change.
- `internal/config/types.go::Repo` is a struct with optional fields — adding
  `Docs *DocsConfig` is non-breaking on existing TOML files.
- `Sync` already iterates repos; a new `syncRepoDocs` step parallels
  `syncRepo`.

### Terminal agent integration

**Goal:** an `ergo agent` subcommand that reads the `ergo` JSON object from
the active `.code-workspace` and routes terminal AI assistants (e.g. Claude
Code, Codex CLI) to the right repo and tags.

**Slot:** the `ergo` JSON object is **already present** in every generated
workspace file — see [`internal/vscode/generate.go`](../../ergo/internal/vscode/generate.go).
Currently it carries:

```jsonc
{
  "ergo": {
    "workspace-name": "ml-projects",
    "tags": ["ml", "go"],
    "filter": { "group": "ml" }
  }
}
```

A future `ergo agent` would read this from the workspace file rather than
re-detecting the workspace, and use the filter to scope its prompt context.

### Task routing

**Goal:** `[workspace.routing]` config that lets users say "the `lint` task
runs `ruff` in python repos and `golangci-lint` in go repos."

**Slot:** would require a new config table plus a small `cmd/task.go` that
expands `ergo task lint` into per-repo `ergo run --tags=python -- ruff` etc.

**Why deferred:** users can already do this with shell aliases or a script
calling `ergo run --tags=...`. The tenet "earn every abstraction" applies.

---

## Phase 10 of the implementation plan: integration suite

Status as of last reading of the plan: **partially landed**.

In place:

- [`test/integration/Dockerfile`](../../ergo/test/integration/Dockerfile)
- [`test/integration/harness/`](../../ergo/test/integration/harness/) (harness, gitfixtures, stubs)

Not yet visible in the tree (per the plan, expected):

- `Makefile` with `make integration` / `make integration-shell`
- `docker-compose.yml`
- The actual `*_test.go` integration tests under `test/integration/`

When complete, the suite will exercise every non-TUI command path
end-to-end against real `git` and stub `gh`/`code` binaries.

### Why TUI is excluded from the docker suite

The plan's decision: TUI testing is done via `teatest` unit tests rather than
in-container, because:

1. The TUI is the same on all platforms — there's nothing OS-specific to test.
2. `teatest` gives golden-file-style assertions over the rendered terminal,
   which is more precise than scraping container output.
3. The shorthand non-interactive equivalents (`ergo add repo <url>`,
   `ergo show <group>`) **are** integration-tested, providing coverage of
   the underlying workflows.

---

## Hardcoded values that could become config

Listed in case they need to flex later:

| Constant                                         | Location                         | Why hardcoded                                   |
| ------------------------------------------------ | -------------------------------- | ----------------------------------------------- |
| `juan7732/ergo` — self-update repo               | `internal/github/github.go`      | One canonical source.                           |
| `ergo-darwin-arm64` — release asset name         | `cmd/update.go` + `release.yml`  | Single supported platform in v1.                |
| `~/.ergo` — global config dir                    | `internal/config/global.go`      | Standard convention.                            |
| `~/ergo-workspaces` — default workspace root     | `[paths].workspace_root` default | Configurable per-user via global TOML.          |
| `4` — default `batch_size`                       | `[parallel].batch_size` default  | Configurable.                                   |
| `root` — name of the always-included root folder | `internal/vscode/generate.go`    | Spec says fixed.                                |
| `ergo` JSON key in `.code-workspace`             | `internal/vscode/generate.go`    | The protocol contract for v2 agent integration. |

---

## Where new commands plug in

To add a new top-level command:

1. Create `cmd/<name>.go` with a `var <name>Cmd = &cobra.Command{...}` and an
   `init()` that calls `rootCmd.AddCommand(<name>Cmd)`.
2. Inside `RunE`, use `resolveWorkspaceName(cmd, args, runner)` from
   [`cmd/validate.go`](../../ergo/cmd/validate.go) to honor the standard
   resolution rules (CWD detection → arg → TUI fallback).
3. Use `filterOptsFromFlags(cmd, globalCfg.Run.ExcludedGroups)` from
   [`cmd/helpers.go`](../../ergo/cmd/helpers.go) if the command takes
   filter flags.
4. Use `tui.RunInline(model)` for any inline prompts (avoid full-screen
   alt-screen for one-shot questions).

Wire-up checklist for a new command that touches the workspace:

- Loads global + workspace config? → `config.LoadGlobal` / `config.LoadWorkspace`.
- Mutates TOML? → `config.WriteWorkspace` (re-marshals everything).
- Generates `.code-workspace`? → `vscode.Generate` + `vscode.WriteIfChanged`.
- Touches state cache? → `workspace.LoadState` / `SaveState` (best-effort).
- Runs git? → take a `git.Runner` (use `execRunner()` in production, fakes in tests).
- Runs `gh`? → `github.Runner`.
- Spawns `code`? → `exec.Command("code", path).Run()`.

Following these conventions keeps the new command consistent with the rest
of the codebase and inherits the safety properties (smart regen, best-effort
state, standard resolution) for free.
