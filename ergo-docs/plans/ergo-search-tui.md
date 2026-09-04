# Implementation Plan: interactive `ergo search` (no-arg TUI + full-index JSON)

Roadmap item: [11-roadmap.md → Theme 1](../11-roadmap.md#theme-1--daily-use).
Builds on the shipped v0.5.0 search (plan: [ergo-search.md](ergo-search.md)).

## Problem

`ergo search` with no arguments currently fails with a bare cobra error
(`Error: accepts 1 arg(s), received 0`). Tenet 1 says an omitted argument
gets a TUI fallback, not an error; `open`, `sync`, `add`, and `show` all
honor it and search is the odd one out. Separately, JSON consumers (agents,
the VS Code extension) have no way to fetch the complete index in one call.

## Decisions already made (2026-09-03, with the user)

1. **No-arg interactive mode is a live filter, not a form.** The full index
   (every repo, folder, workspace) loads once; typing narrows it,
   fzf-style, via the bubbles list component's built-in filtering.
2. **Enter prints the selected hit's absolute path to stdout, exit 0.**
   This mirrors `ergo open --print-dir`: TUI rendering goes to stderr,
   stdout stays clean for command substitution, so the navigation wrapper
   works:

   ```sh
   ergocd-search() { local d; d=$(ergo search) && cd "$d"; }
   ```

3. **`--json` with no query returns the full index.** Same document shape
   with `"query": ""` and every entry. Additive contract change; agents and
   the extension build a complete picker from one invocation.

## Command specification

`Args` changes from `cobra.ExactArgs(1)` to `cobra.MaximumNArgs(1)`.
Dispatch for the no-arg case:

| Invocation                      | Behavior                                                        |
| ------------------------------- | --------------------------------------------------------------- |
| `ergo search <query>`           | Unchanged (v0.5.0 behavior, both modes)                         |
| `ergo search <query> --json`    | Unchanged                                                       |
| `ergo search --json`            | Full index document, `"query": ""`, exit 0. Never a TUI.        |
| `ergo search` (stdin is a TTY)  | Live-filter TUI; Enter prints path to stdout; cancel exits 1    |
| `ergo search` (stdin not a TTY) | Usage error on stderr, exit 1, never hangs                      |

### The TTY gate — gate on STDIN, render to STDERR

This is the load-bearing subtlety. In the wrapper `d=$(ergo search)`,
stdout is captured and is NOT a terminal; stdin and stderr still are. So:

- The interactive gate checks **stdin** only, reusing the
  `stdinIsTerminal` package-var seam from `cmd/add.go` (a var so teatest
  and integration tests can force either path).
- The Bubble Tea program runs with `tea.WithOutput(os.Stderr)` so the TUI
  renders on stderr and stdout carries exactly one line: the chosen path
  (or nothing on cancel).
- Gating on stdout would break the wrapper, the feature's flagship use.
  Add an integration test that runs no-arg search with stdin a pipe and
  asserts the fast usage error; the `add repo` prompt bug (roadmap, Theme
  1) is the cautionary tale for skipping that test.

### TUI behavior

- Inline rendering (`tui.RunInline` conventions; no alt-screen), list
  height capped (~12 rows) with bubbles pagination beyond that.
- One row per hit, compact: `<name>  <kind>  <workspace>  <state>`, using
  the existing kind/state vocabulary (`cloned`/`created`/`synced` etc.).
- Filtering matches the CLI's fields: set each item's `FilterValue()` to
  name + URL + workspace name, so what `ergo search foo` finds, typing
  `foo` in the TUI also finds.
- `// DECISION:` bubbles' default fuzzy filter is acceptable interactive
  divergence from the CLI's plain substring match (friendlier while
  typing; the underlying corpus is identical). Do not build a custom
  substring filter unless fuzzy proves confusing in practice.
- Enter prints the hit's absolute path even when the target is not on disk
  yet (uncloned repo, unsynced workspace); the path is the projected
  location, consistent with the JSON `path` field. `cd` failing on a
  nonexistent directory is honest feedback. Mark `// DECISION:`.
- Cancel (esc / ctrl-c / q): nothing on stdout, exit 1, so `&&` wrappers
  short-circuit. Mark `// DECISION:` and document the wrapper with `&&`
  (not bare `cd "$(ergo search)"`, which would `cd ""` on cancel).

### JSON contract change

`internal/output` needs no struct changes: the document shape is
identical, `query` is `""` for the unfiltered dump. Document in
`03-commands.md` (search section + JSON output contract): an empty query
matches everything, and `""` in the emitted document means "full index".
`internal/workspace.Search` with an empty query should already return all
entries (`strings.Contains(x, "")` is true); verify and add a unit test
pinning that property rather than reimplementing a separate index path.

## Implementation steps

1. **`internal/workspace/search_test.go`**: add the empty-query-returns-
   everything test (behavior likely already correct; pin it).
2. **`internal/tui/search_select.go`** (+ teatest tests): list model over
   `[]workspace.Hit`, filter values as specified, returns the selected hit
   or a cancelled flag. Follow the WorkspaceSelect model's structure and
   styling.
3. **`cmd/search.go`**: `MaximumNArgs(1)`; dispatch table above; TUI path
   runs the model with `tea.WithOutput(os.Stderr)` and prints only the
   path on stdout; non-TTY no-arg prints a one-line usage hint to stderr.
4. **Docs, same PR**: `03-commands.md` (modes table, JSON contract note),
   README (wrapper example next to the existing `ergocd` one; command
   table row gains "interactive with no args"), release notes for the
   next version, roadmap graduation on ship.
5. **Integration tests**: non-TTY no-arg fast error; `--json` no-query
   full index across two fixture workspaces; existing query behavior
   unchanged. TUI itself stays out of the docker suite per the
   established teatest rationale (08-build-test-release.md).

## Out of scope

- Opening/syncing the selection (Enter prints a path; composition does the
  rest — "small verbs").
- A substring/fuzzy toggle or any filter flags on the TUI.
- Cross-device entries (arrives with workspace config sync).
- Any change to the with-query code paths beyond the `Args` relaxation.

## Tenet check

| Tenet                         | How this complies                                             |
| ----------------------------- | ------------------------------------------------------------- |
| Optimize for the hands        | The literal tenet-1 pattern: missing arg → TUI, not an error. |
| Every command is a commitment | No new verbs or flags; one arg becomes optional.              |
| Small verbs, big compositions | Enter emits a path; `cd`, editors, scripts compose it.        |
| Speed is respect              | Index loads once from TOMLs + stats; no git, no network.      |
| Safe by default               | Read-only throughout; cancel exits non-zero, prints nothing.  |
| TOML is truth                 | Purely a read/present layer over existing search.             |

## Size estimate

Small-to-medium: one new TUI model + teatest, modest `cmd/search.go`
dispatch changes, docs. The search engine itself is untouched.
