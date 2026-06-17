# ergo

Multi-repo VS Code workspace manager.

ergo manages development workspaces that span multiple repositories. It clones repos,
organizes them into a working directory, generates VS Code workspace files, and
provides commands to operate across all repos simultaneously. The TOML configuration
file is the single source of truth.

## Requirements

- `git` on PATH
- `gh` (GitHub CLI) on PATH — required for `ergo update`
- `code` (VS Code CLI) on PATH — required for `ergo open` and `ergo edit`

## Install

### Homebrew (recommended)

```bash
brew install juan7732/tap/ergo
```

Updates flow through Homebrew (`brew upgrade ergo`). A Homebrew-managed binary
detects this and `ergo update` defers to `brew upgrade` rather than self-replacing.

### go install

```bash
go install github.com/juan7732/ergo@latest
```

### Prebuilt binary

Download the `ergo-<os>-<arch>` asset for your platform (darwin/linux ×
amd64/arm64) from [Releases](https://github.com/juan7732/ergo/releases), verify
it against the release `checksums.txt`, and place it on your PATH. A standalone
binary installed this way keeps itself current with `ergo update`, which fetches
the matching asset and verifies its SHA-256 before swapping atomically.

## Quickstart

```bash
# 1. Create a workspace definition (guided TUI)
ergo init my-project

# 2. Open it in VS Code (clones repos on first run)
ergo open my-project

# 3. Keep it in sync with your TOML
ergo sync my-project

# 4. Check the state of all repos at a glance
ergo status my-project

# 5. Run a command across all repos
ergo run -- git status
ergo run --tags=go -- go test ./...

# 6. Update ergo itself
ergo update
```

## Commands

| Command                | Description                                                               |
| ---------------------- | ------------------------------------------------------------------------- |
| `ergo init [name]`     | Create a new workspace definition (guided TUI)                            |
| `ergo open [name]`     | Open workspace in VS Code; clones repos on first run                      |
| `ergo sync [name]`     | Sync workspace on disk with TOML config                                   |
| `ergo status [name]`   | Show branch/dirty/behind state for all repos                              |
| `ergo add [name]`      | Add a repo or folder to the workspace                                     |
| `ergo remove [name]`   | Remove a repo or folder from the workspace                                |
| `ergo edit [name]`     | Open the workspace TOML in VS Code (`--global` for `~/.ergo/config.toml`) |
| `ergo list`            | List all configured workspaces                                            |
| `ergo show [group]`    | Filter VS Code view to a group/tag                                        |
| `ergo run -- <cmd>`    | Run a command across all (or filtered) repos                              |
| `ergo validate [name]` | Validate a workspace TOML                                                 |
| `ergo update`          | Check for a new version and update the binary                             |
| `ergo --version`       | Print the current version                                                 |

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
```

### Workspace config (`~/.ergo/workspaces/<name>.toml`)

```toml
[workspace]
name = "my-project"

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

## Filtering

Most commands accept filter flags to operate on a subset of repos:

```bash
--name=<glob>       # Filter by repo name (glob pattern)
--group=<group>     # Filter by group
--tags=<t1,t2>      # Filter by tags (any-match)
```

Filters are AND-ed. `[run].excluded_groups` in global config are automatically
excluded from `ergo run` unless overridden with `--include-group=<group>` or `--all`.

## VS Code Integration

The generated `.code-workspace` file always includes a root folder (`"."`) as the
first entry so VS Code and Copilot can see the workspace root. The file is managed
entirely by ergo — do not edit it by hand.

`ergo show <group>` regenerates the workspace file to include only repos in a
given group, focusing Copilot's context on the work at hand. `ergo show all`
restores the full view.

## Shell `cd` integration

`ergo open --print-dir <name>` prints the workspace directory to stdout instead
of launching VS Code. A child process cannot change the parent shell's cwd, so
wrap it in a function in your shell rc to `cd` after opening:

```sh
ergocd() { cd "$(ergo open --print-dir "$@")"; }
```

First-time clone progress is routed to stderr in this mode so stdout stays
clean for command substitution.

## Development

Build, test, and lint via [`just`](https://github.com/casey/just):

```bash
just --list           # show all recipes
just check            # fmt + vet + race-tested unit suite
just test             # unit tests only
just integration      # dockerized end-to-end suite
just integration-shell  # interactive shell in the integration image
```

The integration suite builds a hermetic container image (Go + git only) and
runs the real `ergo` binary against fixture repos served via `file://` URLs.
The `gh` and `code` shell-outs are stubbed per-test on PATH. Requires Docker
or a compatible runtime (OrbStack, Docker Desktop).
