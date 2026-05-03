# Design Tenets — Distilled

The full text lives in [`token-maximizer/tenets.md`](../../token-maximizer/tenets.md).
Below is a one-line summary of each tenet plus how it shows up in the code.

## 1. Optimize for the hands

Short flags, smart defaults, TUI fallback when an arg is missing instead of an
error message. Consistent flag names across commands.

> Concrete example: `ergo open` and `ergo sync` both accept an optional
> positional `[workspace-name]`. Inside a workspace, no arg is needed.
> Outside one, the WorkspaceSelect TUI launches automatically.

## 2. Earn every abstraction

No speculative interfaces. Pattern must hurt twice before getting a feature.

> Only one preemptive interface in the codebase: `git.Runner` and `github.Runner`,
> and only because they're the seam between ergo and shell processes that need
> to be faked in tests.

## 3. Every command, flag, and config key is a commitment

> Concrete example: `ergo update` hardcodes `juan7732/ergo` as the source repo.
> Making it configurable would be one more knob users have to understand for
> zero benefit.

## 4. Small verbs, big compositions

A small set of precise commands; power through combination with the shell.

> `ergo run --tags=go -- go test ./...` composes ergo's filtering with go's
> test runner. ergo doesn't need a `test` subcommand.

## 5. Complexity is debt

The bar isn't "useful" — it's "does this justify the cognitive load?"

> Reflected in the v1 scope: no doc cache, no agent integration, no task
> routing. All deferred to v2 with explicit hooks (the `ergo` JSON object,
> the `[workspace.routing]` placeholder design).

## 6. Speed is respect

Parallel by default (clone, pull, status, run); cache when possible; never
network when not needed.

> `[parallel].batch_size = 4` is the default. `Sync`, `GatherStatus`, and
> `RunAcrossTargets` all use the same bounded-concurrency pattern.
> `WriteIfChanged` skips no-op writes to keep VS Code from reloading.

## 7. Show your work

Sync reports clone/pull/skip per repo. Run labels output by repo. Errors
include context.

> `cmd/sync.go` prints `✓ <name>  <action>` for every operation. `tui.PrintRunResult`
> prefixes every command's output with a `━━━ <repo> ━━━` banner.

## 8. TOML is truth

The `.code-workspace` is derived. Filesystem is a reflection of the config.

> `vscode.Generate` is pure and deterministic from the TOML. `WriteIfChanged`
> + `os.Rename` ensure the file always reflects the current TOML, never the
> other way around. `ergo show` modifies only the derived file.

## 9. Safe by default, destructive by intent

Sync never deletes. Removing from TOML doesn't delete from disk. Destructive
ops require a flag *and* a confirmation prompt.

> `findOrphans` reports without acting. `--force` adds delete behavior. Every
> destructive code path reads from stdin to confirm.

## 10. Alias-friendly by design

> The README explicitly suggests `alias es="ergo sync"`, `alias er="ergo run --"`.
> Output is parseable in `--short` mode (tab-separated). Exit codes are
> meaningful (any failure → non-zero).

---

# Implementation Discipline

From `.github/copilot-instructions.md`:

- Spec is explicit → follow it.
- Spec is silent → check tenets.
- Tenets don't resolve it → simplest option, mark with `// DECISION:`.
- Genuine architectural gap → stop and describe it.

Markers used in the codebase:

| Marker         | Purpose                                        | Examples                                                                                                     |
| -------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `// DECISION:` | Intentional choice the spec didn't prescribe   | `validate.go` (group="" allowed), `resolve.go` (single match resolves), `group_select.go` (group beats tags) |
| `// REVIEW:`   | Not confident — please check                   | `cmd/list.go` (ANSI padding), `cmd/open.go` (TOML changed but dir exists case)                               |
| `// TODO:`     | Known incomplete, not blocking                 | `state.go` (`CommitHash` unpopulated)                                                                        |
| `// SPEC:`     | References a specific spec section for context | `cmd/run.go` (filter ignored in standalone)                                                                  |

The discipline is consistent enough that a quick `rg "// (DECISION|REVIEW|TODO|SPEC):"` is a reliable map of every place the spec is silent or where future work is acknowledged.
