# ergo

**Agentic context orchestration: git repositories and folders as a declarative
context primitive for coding agents and the humans who work with them.**

ergo is an open-source CLI that lets you declare which repositories, docs, and
working folders make up a *system*, then materializes that declaration into a
multi-repo workspace: cloned, organized, kept in sync, and readable by both
VS Code and terminal agents like Claude Code. One TOML file is the single
source of truth; everything on disk is derived from it and can be rebuilt at
any time.

## The problem

Coding agents are good at file-level work inside a single repository. The hard
remainder, roughly the last 20%, is gated behind context the model was never
trained on and the checkout doesn't contain: the API contract lives in a
second repo, the schema in a third, the deployment config in a fourth, and the
tribal knowledge in nobody's repo at all.

Agents are repo-scoped by default. Systems aren't.

ergo bridges that gap at the filesystem level: it assembles the repos and
folders that constitute a system into one workspace directory, so an agent
launched at its root sees the whole system (upstream contracts, downstream
consumers, and persistent notes) instead of one repo's slice of it.

Consider ergo as an alternative to consolidating your existing repos into a monorepo. Or empowering your existing agents by creating new tooling-as-a-repo primitives.

## Core concept: the workspace as a context primitive

A workspace is a TOML file (`~/.ergo/workspaces/<name>.toml`) declaring:

- **Repos**: the git repositories that belong to the system, each with
  optional `tags` and a `group`.
- **Folders**: free-form directories inside the workspace for notes, scratch
  work, or agent-maintained context; `git = true` makes one version-controlled
  (ergo runs `git init` on sync).
- **Scopes**: groups and tags carve the system into subsystems you can
  filter, focus, and fan out over.

From that declaration, ergo derives everything else:

- The **directory layout**: repos cloned side by side under one root.
- The **`.code-workspace` file**: regenerated deterministically, never
  hand-edited. It embeds an `ergo` JSON block (workspace name plus the active
  view filter) that tools and agents can read to know what context they're in.
- A **machine-readable surface**: `status`, `list`, `config`, `show`, and
  `validate` all take `--json` and emit stable, additive-only documents, so
  agents and tooling script against ergo without parsing human output.

The TOML is truth. Disk state is a projection. That's what makes the context
reproducible: the same declaration materializes the same system on any
machine.

## What it does today

```bash
# Declare a system (guided TUI)
ergo init my-product

# Materialize it: clone everything, generate the workspace, open VS Code
ergo open my-product

# Re-converge disk with the declaration (clone new repos, pull existing)
ergo sync my-product

# One view of the whole system's git state
ergo status my-product

# Fan a command out across the system, or a scope of it
ergo run -- git status
ergo run --tags=go -- go test ./...

# Focus the workspace view (and Copilot's context) on one subsystem
ergo show core
ergo show all

# Machine-readable reads for agents and tooling
ergo config --json
ergo status --json
```

Working with a terminal agent, the pattern is: `ergo open` (or `sync`) to
materialize the system, then launch the agent at the workspace root. The
agent's working directory now *is* the system. Every repo's conventions
files (`CLAUDE.md`, `AGENTS.md`), code, and your notes folders are in scope.

### Command reference

| Command                | Description                                                               |
| ---------------------- | ------------------------------------------------------------------------- |
| `ergo init [name]`     | Create a new workspace definition (guided TUI)                            |
| `ergo open [name]`     | Open workspace in VS Code; clones repos on first run                      |
| `ergo sync [name]`     | Sync workspace on disk with TOML config                                   |
| `ergo status [name]`   | Show branch/dirty/behind state for all repos (`--json`)                   |
| `ergo config [name]`   | Print a workspace's configuration (`--json`)                              |
| `ergo add [name]`      | Add a repo or folder to the workspace                                     |
| `ergo remove [name]`   | Remove a repo or folder from the workspace                                |
| `ergo edit [name]`     | Open the workspace TOML in VS Code (`--global` for `~/.ergo/config.toml`) |
| `ergo list`            | List all configured workspaces (`--json`)                                 |
| `ergo show [group]`    | Filter the workspace view to a group/tag (`--json` to read the filter)    |
| `ergo run -- <cmd>`    | Run a command across all (or filtered) repos                              |
| `ergo validate [name]` | Validate a workspace TOML (`--json`)                                      |
| `ergo update`          | Check for a new version and update the binary                             |
| `ergo --version`       | Print the current version                                                 |

Inside a workspace directory, the `[name]` argument is optional; ergo
detects the workspace from your location. Outside one, omitting it opens a
selector.

## Install

### Homebrew (recommended)

```bash
brew install juan7732/tap/ergo
```

Updates flow through Homebrew (`brew upgrade ergo`). A Homebrew-managed binary
detects this and `ergo update` defers to `brew upgrade` rather than
self-replacing.

### go install

```bash
go install github.com/juan7732/ergo@latest
```

### Prebuilt binary

Download the `ergo-<os>-<arch>` asset for your platform (darwin / linux /
windows × amd64 / arm64; Windows assets carry a `.exe` suffix) from
[Releases](https://github.com/juan7732/ergo/releases), verify it against the
release `checksums.txt`, and place it on your PATH. A standalone binary keeps
itself current with `ergo update`, which fetches the matching asset and
verifies its SHA-256 before swapping it in.

> **Windows:** Homebrew is macOS/Linux only; install via the prebuilt `.exe`.
> A `winget` channel is on the roadmap.

Requirements: `git` on PATH; `gh` (GitHub CLI) for `ergo update`; `code`
(VS Code CLI) for `ergo open` and `ergo edit`.

## Configuration

### Global config (`~/.ergo/config.toml`)

Created automatically on first run with defaults:

```toml
[defaults]
workspace_root = "~/ergo-workspaces"
default_branch = "main"

[parallel]
enabled = true
batch_size = 4

[sync]
auto_pull = true

[run]
excluded_groups = []

[git]
protocol = "https"
```

Set `protocol = "ssh"` if you authenticate to git hosts with SSH keys: ergo
then rewrites `https://` repo URLs to SSH form (`git@host:owner/repo.git`)
in memory at clone time. The stored workspace TOML is never modified.

### Workspace config (`~/.ergo/workspaces/<name>.toml`)

```toml
[workspace]
name = "my-product"

[[repos]]
url = "https://github.com/you/backend.git"
tags = ["go"]
group = "core"

[[repos]]
url = "https://github.com/you/frontend.git"
tags = ["ts"]
group = "core"

[[folders]]
name = "scratch"

[[folders]]
name = "notes"
git = true
```

### Filtering

Most commands accept filter flags to operate on a subset of the system:

```bash
--name=<glob>       # Filter by repo name (glob pattern)
--group=<group>     # Filter by group
--tags=<t1,t2>      # Filter by tags (any-match)
```

Filters are AND-ed. `[run].excluded_groups` in global config are automatically
excluded from `ergo run` unless overridden with `--include-group=<group>` or
`--all`.

### Shell `cd` integration

`ergo open --print-dir <name>` prints the workspace directory to stdout
instead of launching VS Code. Useful for dropping a shell (or an agent
session) at the system root:

```sh
ergocd() { cd "$(ergo open --print-dir "$@")"; }
```

First-time clone progress is routed to stderr in this mode so stdout stays
clean for command substitution.

## How it works

ergo is a single Go binary with no daemon and no state beyond your TOML files
and a best-effort cache. The design is deliberately boring:

- **Declarative core.** Workspace generation is a pure function of the TOML.
  Generated files are written through a change-detecting writer, so no-op
  syncs don't touch disk (and don't make VS Code reload).
- **Parallel by default.** Clone, pull, status, and `run` fan out with
  bounded concurrency (`batch_size`, default 4).
- **Safe by default.** `sync` never deletes; removing a repo from the TOML
  leaves its directory on disk. Destructive operations require an explicit
  flag *and* a confirmation.
- **Stable machine surface.** The `--json` documents are a versioned,
  additive-only contract: fields never change or disappear once shipped.
  External tooling (including the in-development VS Code extension) builds
  against it rather than against ergo's internals.

Full documentation (architecture, command semantics, operational guarantees,
and design tenets) lives in [`ergo-docs/`](ergo-docs/00-overview.md).

## Roadmap

Tracked in [`ergo-docs/11-roadmap.md`](ergo-docs/11-roadmap.md). Not yet
shipped; listed here so current capabilities above stay unambiguous:

- **`ergo search <query>`**: find which workspaces reference a repo, folder,
  or name, with on-disk state per hit.
- **VS Code extension** ([ergo-vscode](https://github.com/juan7732/ergo-vscode)):
  sidebar and status-bar surface over ergo's JSON contract. In development,
  read-only milestone working; not yet published.
- **Sandboxed agent-runtime exploration**: ergo as the substrate that
  materializes read-only context spaces plus agent-managed working folders
  for autonomous agents; includes evaluating an MCP adapter over the JSON
  contract where a CLI can't reach.
- **`winget` distribution.**

## Development

Build, test, and lint via [`just`](https://github.com/casey/just):

```bash
just --list           # show all recipes
just check            # fmt + vet + race-tested unit suite
just test             # unit tests only
just integration      # dockerized end-to-end suite
```

The integration suite builds a hermetic container image (Go + git only) and
runs the real `ergo` binary against fixture repos served via `file://` URLs.
The `gh` and `code` shell-outs are stubbed per-test on PATH. Requires Docker
or a compatible runtime (OrbStack, Docker Desktop).

## License

[MIT](LICENSE)
