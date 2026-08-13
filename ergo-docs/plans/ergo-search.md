# Implementation Plan: `ergo search <query>`

Roadmap item: [11-roadmap.md → Theme 1](../11-roadmap.md#theme-1--daily-use).

## Problem

"Do I already have repo X pulled into one of my workspaces?" Today the only
answer is grepping `~/.ergo/workspaces/*.toml` by hand, and the question gets
harder the more workspaces exist. One command should answer it completely:
which workspace(s) reference the thing, and whether it's actually on disk.

## Command specification

```
ergo search <query>          # exactly one positional arg
ergo search <query> --json   # machine-readable document
```

- **Scope:** searches *all* configured workspaces — a global command like
  `ergo list`. No workspace resolution, no filter flags.
- **What is matched:** case-insensitive substring match of `<query>` against
  - repo **effective name** (`Repo.EffectiveName()`) and **URL**,
  - folder **name**,
  - workspace **name**.
- **Reported per hit:** workspace, kind (`repo`/`folder`/`workspace`), name,
  group + tags + URL (repos), and on-disk state with the absolute path.

DECISION candidates to mark in code (spec is silent, tenets don't force):

- *Substring, not glob.* The `--name` filter flag uses globs, but search's
  job is recall ("is `ergo` anywhere?"), where `foo*bar` precision is
  unhelpful and forgetting to add `*` around the query is a footgun. Mark
  `// DECISION:` in the matcher; glob support is additive later if it hurts.
- *No hits → exit 0.* An empty result is a successful query, consistent with
  `ergo list --json` printing `{"workspaces": []}` with exit 0 and with
  "filter matching nothing yields `[]`, exit 0" in `status --json`. Scripts
  test emptiness via `jq -e '.results | length > 0'`. (grep-style exit 1 is
  the plausible alternative — flag as `// DECISION:` for review.)
- *Unreadable workspace TOMLs* are skipped with a warning on stderr, exactly
  like `runList` ([cmd/list.go](../../cmd/list.go)) — a broken workspace must
  not hide hits in healthy ones, and `--json` stdout stays clean.

### On-disk state semantics

Mirrors what already exists — no new definitions:

| Kind      | State field | True when                                                          |
| --------- | ----------- | ------------------------------------------------------------------ |
| repo      | `cloned`    | `<wsRoot>/<ws>/<name>/.git` exists (same check as `manager.go` `isCloned`) |
| folder    | `created`   | `<wsRoot>/<ws>/<name>` exists as a directory                       |
| workspace | `synced`    | `<wsRoot>/<ws>` exists as a directory (same as `ergo list`)        |

### Human output

Flat table in `ergo list`'s style (lipgloss borders), one row per hit,
ordered by workspace name then kind then name:

```
┌────────────┬────────┬──────┬───────┬────────────┬──────────┐
│ Workspace  │ Kind   │ Name │ Group │ Tags       │ State    │
├────────────┼────────┼──────┼───────┼────────────┼──────────┤
│ ml-projects│ repo   │ ergo │ core  │ go         │ cloned   │
│ platform   │ repo   │ ergo │       │            │ uncloned │
└────────────┴────────┴──────┴───────┴────────────┴──────────┘
```

No hits (human mode): `no matches for "<query>"` on stdout, exit 0.

### JSON contract (`--json`)

New document in `internal/output`, following the package's rules (dedicated
wire structs, explicit constructor, additive-only evolution, `[]` never
`null`):

```json
{
  "query": "ergo",
  "results": [
    {
      "workspace": "ml-projects",
      "kind": "repo",
      "name": "ergo",
      "url": "https://github.com/juan7732/ergo.git",
      "group": "core",
      "tags": ["go"],
      "cloned": true,
      "path": "/Users/jrv/ergo-workspaces/ml-projects/ergo"
    },
    { "workspace": "scratchpad", "kind": "folder", "name": "ergo-notes",
      "created": true, "path": "/Users/jrv/ergo-workspaces/scratchpad/ergo-notes" },
    { "workspace": "ergo-ecosystem", "kind": "workspace", "name": "ergo-ecosystem",
      "synced": true, "path": "/Users/jrv/ergo-workspaces/ergo-ecosystem" }
  ]
}
```

- `kind` discriminates the entry; kind-specific fields (`url`, `group`,
  `tags`, `cloned`/`created`/`synced`) use `omitempty` where absent-by-kind,
  but fields that exist for a kind are always emitted (`tags: []`, not
  omitted).
- `path` is the absolute on-disk location whether or not it exists yet —
  it's the answer to the follow-up question ("take me there" / agent `cd`),
  and consumers must not reimplement `wsRoot` join logic.
- No hits → `{"query": "...", "results": []}`, exit 0.

## Implementation steps

Follows the new-command checklist in
[10-extension-points.md → Where new commands plug in](../10-extension-points.md#where-new-commands-plug-in).

1. **`internal/workspace/search.go`** — pure matching logic:
   `Search(query string, workspaces []NamedConfig, wsRoot string) []Hit`
   where `Hit` carries kind, workspace, name, url/group/tags, and the
   computed path + exists/cloned state. Filesystem probes behind the same
   pattern `status.go` uses so tests can drive them with `t.TempDir()`.
   Unit tests: `search_test.go` — case-insensitivity, URL matching,
   effective-name derivation (explicit `name` vs URL-derived), folder and
   workspace-name hits, multi-workspace ordering, empty results.

2. **`internal/output/output.go`** — `SearchResult` + `Search` structs and
   `NewSearch(query string, hits []workspace.Hit) Search` constructor, with
   tests in `output_test.go` (nil → `[]` normalization, per-kind field
   presence).

3. **`cmd/search.go`** — cobra command: `Args: cobra.ExactArgs(1)`, `--json`
   flag, `config.LoadGlobal()` → `ExpandTilde(WorkspaceRoot)`,
   `config.ListWorkspaceNames()` + `LoadWorkspace` loop with stderr warnings
   for unreadable TOMLs (copy the `runList` pattern), then either
   `printJSON(cmd, output.NewSearch(...))` or the table renderer. Table
   rendering reuses `tui.StyleTableBorder`/`StyleTableHeader` — note
   list.go's `// REVIEW:` about ANSI padding widths applies here too; the
   State column should style *after* width computation or use
   `lipgloss.Width`.

4. **Docs** — in the same PR:
   - `03-commands.md`: new `## ergo search <query>` section + an entry in
     the JSON output contract section.
   - `README.md`: row in the Commands table.
   - `release-notes/v0.5.0.md` (create if absent): `## New: ergo search`
     entry per the release-notes structure.
   - `11-roadmap.md`: remove the item when it ships.

5. **Integration test** — `test/integration/`: real binary in the hermetic
   harness with two fixture workspaces; assert hits across workspaces, the
   `cloned` bit flipping after `ergo sync`, clean `--json` stdout with a
   corrupt third workspace TOML (warning on stderr, exit 0), and the
   no-match exit code.

## Explicitly out of scope (v1)

- **Content search inside repos** — that's ripgrep's job; "small verbs, big
  compositions" (`ergo run -- rg <pattern>` already composes).
- **Glob/regex queries** — additive later if substring hurts.
- **Filter flags** (`--group`, `--tags`) — search is recall across
  everything; filters are for scoped operations.
- **Cross-device search** — once workspace config sync lands
  ([11-roadmap.md](../11-roadmap.md), Theme 1), search extends over the
  local sync clone to answer "which of my machines has this workspace?"
  offline. Additive change: a `device` field on results. Nothing in this
  plan blocks it; the flat `results` array and kind discriminator were
  chosen with it in mind.
- **Extension integration** — ergo-vscode can consume `search --json` later
  (e.g. quick-pick "which workspace has X?"); nothing in this plan blocks
  or requires it.

## Tenet check

| Tenet                        | How this complies                                              |
| ---------------------------- | -------------------------------------------------------------- |
| Earn every abstraction       | No new interfaces; pure function + existing loaders.           |
| Every command is a commitment| One verb, one positional, one flag (`--json`).                 |
| Small verbs, big compositions| Finds things; acting on them composes via shell/`jq`/`path`.   |
| Speed is respect             | Reads only TOMLs + `os.Stat` — no git, no network.             |
| Show your work               | Per-hit disk state; skipped-workspace warnings on stderr.      |
| TOML is truth                | Searches configs, reports disk as derived state, mutates nothing. |

## Size estimate

Small: ~4 new files (`cmd/search.go`, `internal/workspace/search.go` + test,
integration test), one addition to `internal/output`, docs. Comparable to the
`ergo config --json` addition in scope.
