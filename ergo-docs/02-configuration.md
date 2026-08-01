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

[git]
protocol = "https"                      # "ssh" rewrites https repo URLs to SSH form at clone time
```

Implementation:

- `LoadGlobal()` reads the file or writes defaults if it doesn't exist.
- `defaultGlobalConfig()` returns the canonical defaults shown above.
- File is created with mode `0600`, parent dir with `0700`.

### `[git].protocol`

Controls the transport used for ergo-initiated clones:

- `"https"` (default, also used when the key or section is absent): repo URLs
  are passed to `git clone` exactly as written in the workspace TOML. No
  rewriting in either direction — scp-style URLs stay scp-style.
- `"ssh"`: `https://` / `http://` URLs are rewritten to scp form
  (`https://github.com/o/r.git` → `git@github.com:o/r.git`) **in memory at
  clone time only**. The stored workspace TOML is never modified. Works for any
  host (GitHub, GitLab, Bitbucket, self-hosted). URLs that cannot be safely
  rewritten pass through unchanged: URLs with an explicit port or embedded
  credentials, and anything that is not a plain http(s) URL (`ssh://`, `git://`,
  `file://`, local paths, already-scp forms). Implemented by `RewriteToSSH` in
  [`internal/git/url.go`](../../ergo/internal/git/url.go).

Notes:

- **Existing clones keep their original `origin` remote** — the rewrite applies
  at clone time only, and `sync` pulls via the clone's configured remote. To
  migrate an existing checkout, run
  `git remote set-url origin git@github.com:owner/repo.git` in the repo, or
  delete the directory and re-sync.
- Ergo-initiated git commands run with `GIT_TERMINAL_PROMPT=0`, so a repo that
  needs credentials fails fast with a per-repo error and a remediation hint
  (instead of a hung username prompt during parallel sync). Non-interactive
  credential helpers (VS Code, git-credential-manager) are unaffected.
- Rare edge: an SSH key with a passphrase and no ssh-agent will still make git
  wait on the ssh prompt; add the key to your agent (`ssh-add`) to avoid this.

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

| Field             | Required | Type     | Default                     | Notes                                                                                                                |
| ----------------- | -------- | -------- | --------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `url`             | yes      | string   | —                           | git clone URL                                                                                                        |
| `name`            | no       | string   | `DeriveRepoName(url)`       | only needed to disambiguate collisions                                                                               |
| `branch`          | no       | string   | `[defaults].default_branch` | passed to `git clone --branch`; falls back to the remote default if the named branch doesn't exist (e.g. empty repo) |
| `tags`            | no       | []string | `[]`                        | filter targets, any-match semantics                                                                                  |
| `group`           | no       | string   | `""` (no group)             | one group per repo; used by `--group`, excluded_groups                                                               |
| `vscode_settings` | no       | table    | `{}`                        | merged into folder entry in `.code-workspace`                                                                        |

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

## Derived artifact — `<name>.code-workspace`

The workspace file is generated from the TOML (`vscode.Generate`) and never
edited by hand. Its top-level `ergo` object carries two pieces of metadata:

- `workspace-name` — maps the file back to `~/.ergo/workspaces/<name>.toml`.
- `filter` — the active `ergo show` view filter (`{group}`, `{tags}`, or
  `{name}`), present only while a filter is active.

`filter` is durable state: `sync` and `open` read it back
(`vscode.ReadFilter`) and re-apply it when regenerating, so the filtered view
survives until `ergo show all` clears it. It affects only the generated
folders list — never which repos are synced, validated, or deleted. See
[07-operational-semantics.md](07-operational-semantics.md#show-filter-preservation).

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
