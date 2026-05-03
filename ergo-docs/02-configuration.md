# Configuration Reference

ergo has two TOML files: a **global** config and one **workspace** config per
workspace. There is also a JSON state cache that users do not interact with directly.

## Global config — `~/.ergo/config.toml`

Created automatically with defaults on first run if missing. Defined in
[`internal/config/global.go`](../../ergo/internal/config/global.go).

```toml
[defaults]
workspace_root = "~/ergo-workspaces"   # parent dir for materialized workspaces
default_branch = "main"                # used when [[repos]].branch is unset

[parallel]
enabled    = true                       # master switch for concurrent ops
batch_size = 4                          # max concurrent clone/pull/run/status

[sync]
auto_pull = true                        # if false, sync only clones missing repos

[run]
excluded_groups = []                    # groups skipped by `ergo run` unless overridden
```

Implementation:

- `LoadGlobal()` reads the file or writes defaults if it doesn't exist.
- `defaultGlobalConfig()` returns the canonical defaults shown above.
- File is created with mode `0600`, parent dir with `0700`.

The exported helpers `config.ErgoHome()` and `config.ExpandTilde(path)` are
used by callers outside the package (e.g. [`cmd/edit.go`](../../ergo/cmd/edit.go),
[`internal/workspace/state.go`](../../ergo/internal/workspace/state.go)).

## Workspace config — `~/.ergo/workspaces/<name>.toml`

Defined in [`internal/config/workspace.go`](../../ergo/internal/config/workspace.go)
and [`types.go`](../../ergo/internal/config/types.go).

```toml
[workspace]
name = "ml-projects"

# Optional workspace-wide VS Code settings (top-level "settings" in .code-workspace)
[workspace.vscode.settings]
"editor.formatOnSave"      = true
"editor.defaultFormatter"  = "esbenp.prettier-vscode"

[[repos]]
url    = "https://github.com/juan/handwriting-recognition.git"
# name auto-derived from URL path component before .git
branch = "dev"                          # optional; defaults to [defaults].default_branch
tags   = ["ml", "python"]
group  = "ml"

# Per-folder VS Code settings → folder-level "settings" in .code-workspace
[repos.vscode_settings]
"python.defaultInterpreterPath" = ".venv/bin/python"

[[repos]]
url   = "https://github.com/juan/ergo.git"
tags  = ["tools", "go"]
group = "tools"

[[repos]]
url  = "https://github.com/other-org/utils.git"
name = "utils-other"                    # explicit name to avoid collision

[[folders]]
name = "scratch"

[[folders]]
name = "planning"
git  = true                             # ergo will run `git init` on first sync
```

### `[[repos]]` field reference

| Field             | Required | Type     | Default                     | Notes                                                  |
| ----------------- | -------- | -------- | --------------------------- | ------------------------------------------------------ |
| `url`             | yes      | string   | —                           | git clone URL                                          |
| `name`            | no       | string   | `DeriveRepoName(url)`       | only needed to disambiguate collisions                 |
| `branch`          | no       | string   | `[defaults].default_branch` | passed to `git clone --branch`                         |
| `tags`            | no       | []string | `[]`                        | filter targets, any-match semantics                    |
| `group`           | no       | string   | `""` (no group)             | one group per repo; used by `--group`, excluded_groups |
| `vscode_settings` | no       | table    | `{}`                        | merged into folder entry in `.code-workspace`          |

### `[[folders]]` field reference

| Field             | Required | Type   | Default | Notes                                             |
| ----------------- | -------- | ------ | ------- | ------------------------------------------------- |
| `name`            | yes      | string | —       | created at `<workspace_root>/<workspace>/<name>/` |
| `git`             | no       | bool   | `false` | run `git init` if not already a git repo          |
| `vscode_settings` | no       | table  | `{}`    | folder-level `.code-workspace` settings           |

### Name derivation

`DeriveRepoName(url)` in
[`internal/config/workspace.go`](../../ergo/internal/config/workspace.go) handles
both HTTPS and scp-style git URLs:

| Input                                                 | Output                    |
| ----------------------------------------------------- | ------------------------- |
| `https://github.com/juan/handwriting-recognition.git` | `handwriting-recognition` |
| `git@github.com:juan/my-tool.git`                     | `my-tool`                 |
| `https://github.com/owner/repo`                       | `repo`                    |
| `https://github.com/owner/repo.git/`                  | `repo`                    |

Algorithm: trim trailing `/`, trim trailing `.git`, take everything after the
last `/` or `:`.

### Collision rules

Implemented in `Validate()` ([`validate.go`](../../ergo/internal/config/validate.go)):

- **Repo↔repo:** two `[[repos]]` whose effective names match → error pointing
  to both indices and asking for an explicit `name`.
- **Folder↔folder:** duplicate folder names → error.
- **Folder↔repo:** a `[[folders]].name` matching any repo's effective name → error.

Other validation:

- `repos[i].url` must be present.
- Tags entries must be non-empty strings if present (empty `group` is allowed
  and means "no group" — explicit `// DECISION:` comment in `validate.go`).
- Folders must have non-empty `name`.

`Validate` returns `ValidationErrors` (an alias for `[]ValidationError`)
implementing `error`. Callers use `errors.As` to retrieve every issue.

## State cache — `~/.ergo/state/<workspace>.json`

Performance optimization, **not** authoritative. Defined in
[`internal/workspace/state.go`](../../ergo/internal/workspace/state.go).

```json
{
  "workspace": "ml-projects",
  "last_sync": "2026-04-30T10:30:00Z",
  "repos": {
    "handwriting-recognition": {
      "last_pulled": "2026-04-30T10:30:00Z",
      "commit_hash": ""
    }
  }
}
```

Behavior:

- `LoadState(name)` returns a zero-value struct **without error** when the file
  is missing or contains invalid JSON. This is intentional — the cache is
  best-effort and ergo must work without it.
- `SaveState(state)` is called by `open` and `sync` after each successful run.
- `commit_hash` is currently unpopulated (TODO in source — the value is not yet
  consumed by any reader).

## Path conventions

| Path                                             | Owner / Mode | Created by                            |
| ------------------------------------------------ | ------------ | ------------------------------------- |
| `~/.ergo/`                                       | dir `0700`   | `LoadGlobal` on demand                |
| `~/.ergo/config.toml`                            | file `0600`  | `LoadGlobal` if missing               |
| `~/.ergo/workspaces/`                            | dir `0700`   | `WriteWorkspace` on demand            |
| `~/.ergo/workspaces/<name>.toml`                 | file `0600`  | `init`, `add`, `remove`, `sync --add` |
| `~/.ergo/state/<name>.json`                      | file `0600`  | `SaveState` after sync/open           |
| `~/ergo-workspaces/<name>/`                      | dir `0700`   | `Sync` on first materialization       |
| `~/ergo-workspaces/<name>/<name>.code-workspace` | file `0600`  | `Generate` + `WriteIfChanged`         |
