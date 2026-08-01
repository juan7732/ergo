# ergo - Personal Workspace CLI Specification

> **Language:** Go
> **UI:** Bubble Tea TUI + standard CLI
> **Status:** Specification v2.0
> **Author:** Juan
> **Purpose:** Lightweight workspace management tool that structures context for AI-assisted
> development across VS Code, terminal agents, and local/cloud LLMs.
> **Prerequisites:** `git`, `gh`, and `code` on PATH.

---

## 1. Overview

### What ergo Does

ergo manages multi-repo development workspaces. It clones repos, organizes them into
a working directory, generates VS Code workspace files, and provides commands to operate
across all repos simultaneously. The TOML configuration file is the single source of truth.

### Core Principles

See `TENETS.md` for the full design philosophy. The essentials:

1. **TOML is truth.** The `.code-workspace` file is a derived output.
2. **Repos are context.** Pulling repos into a workspace gives AI tools deep understanding.
3. **Small verbs, big compositions.** A small set of precise commands; power through combination.
4. **Safe by default, destructive by intent.** Normal operations never cause data loss.
5. **Speed is respect.** Parallel by default, progress when waiting, instant when cached.

### Design Boundaries (v1)

**In scope:**
- Workspace TOML config management
- Git clone/pull via shell
- VS Code `.code-workspace` generation with root folder
- Cross-repo command execution (`ergo run`)
- Repo tagging, grouping, filtering, and group exclusion
- Non-repo folders (optional git init)
- Workspace detection (run from anywhere)
- Status overview (branch, dirty state, behind/ahead)
- Bubble Tea TUI for interactive flows
- View filtering (`ergo show`)
- Self-update (`ergo update`)

**Out of scope for v1 (design for later):**
- Doc cache generation (v2 candidate)
- Terminal agent integration beyond VS Code (v2 candidate)
- Task routing between local/cloud models (v2 candidate)

---

## 2. Tool Semantics

### Workspace Detection

ergo can be run from anywhere. Its behavior adapts based on context:

**Inside an ergo workspace:**
- ergo detects the workspace by walking up the directory tree looking for a
  `.code-workspace` file containing an `"ergo"` key, or by matching the CWD
  against known workspace roots from `~/.ergo/workspaces/*.toml`
- All commands that operate on a workspace use the detected workspace automatically
- The shorthand `"."` always refers to the current workspace explicitly

**Outside an ergo workspace (but inside a git repo):**
- ergo acts on that single repo directly where applicable
- Commands like `ergo status` show status for just that repo
- Commands like `ergo run -- <cmd>` execute in just that repo

**Outside any workspace or repo:**
- Commands that require a workspace launch the TUI selector
- Commands like `ergo list` work normally

### Workspace Resolution Order

For any command that accepts an optional `[workspace-name]`:

1. If a name is provided and matches exactly → use it
2. If a name is provided and partially matches → TUI with filtered list
3. If a name is provided and nothing matches → error with suggestion
4. If `"."` is provided → use current workspace (error if not in one)
5. If no name is provided and CWD is in a workspace → use it
6. If no name is provided and CWD is not in a workspace → TUI selector

---

## 3. Directory Structure

### Global ergo Home

```
~/.ergo/
├── config.toml              # Global config (defaults, parallel settings, excluded groups)
├── state/                   # Cached state (last sync times, repo metadata)
│   └── <workspace-name>.json
└── workspaces/              # Workspace definition files
    ├── knowledge-base.toml
    ├── ml-projects.toml
    ├── startup.toml
    └── side-projects.toml
```

### Workspace Root (on disk)

Default location: `~/ergo-workspaces/` (configurable in global config)

```
~/ergo-workspaces/
└── <workspace-name>/
    ├── <workspace-name>.code-workspace   # Generated - do not edit
    ├── repo-a/                           # Cloned repo
    ├── repo-b/                           # Cloned repo
    ├── repo-c/                           # Cloned repo
    ├── scratch/                          # Non-repo folder (defined in TOML)
    └── planning/                         # Non-repo folder (git-initialized if configured)
```

Everything is flat inside the workspace root. Repos and folders are siblings.
No nesting, no grouping by tag on disk. Tags are a logical concept in the TOML,
not a filesystem concept.

---

## 4. Configuration Format

### Global Config (`~/.ergo/config.toml`)

```toml
[defaults]
workspace_root = "~/ergo-workspaces"     # Where workspace folders are created
default_branch = "main"                   # Default branch to clone/track

[parallel]
enabled = true                            # Master switch for parallel operations
batch_size = 4                            # Max concurrent operations (clone, pull, run)

[sync]
auto_pull = true                          # Whether sync does git fetch + pull on existing repos
                                          # Set to false to make sync only clone missing repos

[run]
excluded_groups = ["documentation", "tools"]   # Groups excluded from `ergo run` by default
                                               # Override with --include-group=<group> or --all
```

### Workspace Config (`~/.ergo/workspaces/<name>.toml`)

```toml
[workspace]
name = "ml-projects"

# VS Code workspace generation settings (optional)
[workspace.vscode.settings]
"editor.formatOnSave" = true
"editor.defaultFormatter" = "esbenp.prettier-vscode"

# Repos
[[repos]]
url = "https://github.com/juan/handwriting-recognition.git"
# name is auto-derived as "handwriting-recognition" from the URL
branch = "dev"                           # Optional, defaults to global default_branch
tags = ["ml", "python"]
group = "ml"

# Per-folder VS Code settings override (optional)
[repos.vscode_settings]
"python.defaultInterpreterPath" = ".venv/bin/python"

[[repos]]
url = "https://github.com/juan/ergo.git"
tags = ["tools", "go"]
group = "tools"

[[repos]]
url = "https://github.com/juan/kb-core.git"
tags = ["kb"]
group = "documentation"

[[repos]]
url = "https://github.com/juan/kb-python.git"
tags = ["kb"]
group = "documentation"

[[repos]]
# Disambiguating: two repos that would derive the same name
url = "https://github.com/juan/utils.git"
name = "utils-juan"                      # Explicit name to avoid collision
tags = ["shared"]

[[repos]]
url = "https://github.com/other-org/utils.git"
name = "utils-other"
tags = ["shared"]

# Non-repo folders
[[folders]]
name = "scratch"

[[folders]]
name = "planning"
git = true                               # ergo will run git init in this folder
```

### Field Reference

**`[[repos]]` fields:**

| Field             | Required | Type     | Default                                  | Description                                                     |
| ----------------- | -------- | -------- | ---------------------------------------- | --------------------------------------------------------------- |
| `url`             | Yes      | string   | -                                        | Git clone URL                                                   |
| `name`            | No       | string   | Derived from URL (portion before `.git`) | Directory name on disk. Only needed to disambiguate collisions. |
| `branch`          | No       | string   | `[defaults].default_branch`              | Branch to checkout                                              |
| `tags`            | No       | string[] | `[]`                                     | Arbitrary tags for filtering                                    |
| `group`           | No       | string   | `""`                                     | Logical group. One group per repo.                              |
| `vscode_settings` | No       | table    | `{}`                                     | VS Code settings overrides for this folder                      |

**Name derivation:** `https://github.com/juan/handwriting-recognition.git` → `handwriting-recognition`.
If two repos derive the same name, ergo errors during validation and asks the user to
provide explicit `name` fields to disambiguate.

**`[[folders]]` fields:**

| Field             | Required | Type   | Default | Description                                            |
| ----------------- | -------- | ------ | ------- | ------------------------------------------------------ |
| `name`            | Yes      | string | -       | Directory name, created inside workspace root          |
| `git`             | No       | bool   | `false` | If true, ergo runs `git init` when creating the folder |
| `vscode_settings` | No       | table  | `{}`    | VS Code settings overrides for this folder             |

**Folders and `ergo run`:** Folders are excluded from `ergo run` by default.
They are non-runnable unless explicitly included via `--include-folders`.

---

## 5. Generated `.code-workspace` File

### Root Folder Requirement

Every generated `.code-workspace` file includes a root folder entry as the first
folder. This is baked-in behavior, not configurable. It ensures VS Code and Copilot
can always see the workspace root, including the `.code-workspace` file itself and
any top-level files.

### Example Output

```json
{
  "ergo": {
    "workspace-name": "ml-projects"
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "handwriting-recognition",
      "path": "handwriting-recognition"
    },
    {
      "name": "ergo",
      "path": "ergo"
    },
    {
      "name": "kb-core",
      "path": "kb-core"
    },
    {
      "name": "kb-python",
      "path": "kb-python"
    },
    {
      "name": "utils-juan",
      "path": "utils-juan"
    },
    {
      "name": "utils-other",
      "path": "utils-other"
    },
    {
      "name": "scratch",
      "path": "scratch"
    },
    {
      "name": "planning",
      "path": "planning"
    }
  ],
  "settings": {
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  }
}
```

The `"ergo"` object at the top level is a custom field that VS Code ignores but
allows ergo (and any other tooling) to identify which workspace config generated
this file and read the corresponding TOML. The `workspace-name` value maps to the
TOML filename in `~/.ergo/workspaces/`.

### Generation Rules

1. Root folder (`"name": "root", "path": "."`) is always first. Non-negotiable.
2. Repos appear next, in the order defined in the TOML.
3. Folders appear after all repos.
4. Workspace-level `[workspace.vscode.settings]` maps to top-level `"settings"`.
5. Per-repo/folder `vscode_settings` maps to folder-level settings.
6. **Smart regeneration:** ergo computes the expected `.code-workspace` content
   and compares it to what's on disk. If identical, the file is not rewritten.
   This avoids unnecessary file change events in VS Code and preserves timestamps.
7. When `ergo show` is active (see §6.10), the generated file reflects the
   filtered view, and the `"ergo"` object includes filter state.

### `.code-workspace` with Active Filter

When `ergo show` is filtering the workspace, the ergo object records the filter
so other tools can understand the current view:

```json
{
  "ergo": {
    "workspace-name": "ml-projects",
    "filter": {
      "type": "group",
      "value": "ml"
    }
  },
  "folders": [
    {
      "name": "root",
      "path": "."
    },
    {
      "name": "handwriting-recognition",
      "path": "handwriting-recognition"
    }
  ]
}
```

---

## 6. Commands

### 6.1 `ergo init`

Create a new workspace definition.

```
ergo init <workspace-name>
```

**Behavior:**
1. Launches guided TUI flow:
   - Prompt for workspace name (pre-filled if provided as arg)
   - Repo entry loop:
     - Prompt for repo URL
     - Prompt for name (default: derived from URL, Enter to skip)
     - Prompt for branch (default: `main`, Enter to skip)
     - Repeat until user submits a blank URL
   - Folder entry loop:
     - Prompt for folder name
     - Prompt for git initialized? (default: No)
     - Repeat until user submits a blank name
2. Writes `~/.ergo/workspaces/<workspace-name>.toml`
3. Does NOT create the workspace directory or clone anything

The workspace now appears in `ergo list` and can be materialized with `ergo open`.

---

### 6.2 `ergo open`

Open a workspace in VS Code. Creates the workspace on disk if it doesn't exist yet.

```
ergo open [workspace-name]
```

**Behavior:**
1. Resolve workspace (see §2 Workspace Resolution Order)
2. If workspace directory doesn't exist on disk:
   - Create the directory at `<workspace_root>/<workspace-name>/`
   - Run full sync (clone repos, create folders)
   - Generate `.code-workspace` file
3. If workspace directory exists:
   - Regenerate `.code-workspace` only if content has changed (smart regeneration)
4. Run `code <workspace-name>.code-workspace`

**`ergo open` is the primary entry point for first-time workspace creation.** `ergo init`
defines the workspace, `ergo open` materializes it.

---

### 6.3 `ergo sync`

Synchronize the workspace on disk with the TOML configuration.

```
ergo sync [workspace-name]
```

**Behavior:**
1. Resolve workspace
2. For each `[[repos]]` in TOML:
   - If directory doesn't exist → `git clone`
   - If directory exists and `[sync].auto_pull` is true → `git fetch && git pull`
   - If directory exists and `auto_pull` is false → skip
3. For each `[[folders]]` in TOML:
   - If directory doesn't exist → create it
   - If `git = true` and directory is not a git repo → `git init`
4. Regenerate `.code-workspace` (smart - only if changed)
5. Warn about orphaned directories (exist on disk but not in TOML)
6. Display summary

**Sync never deletes.** Repos or folders removed from the TOML remain on disk.
Sync warns about them but does not touch them.

**Flags:**
- `--force`: Delete any repo or folder on disk that is not in the TOML.
  Requires confirmation prompt before proceeding.
- `--add`: Reverse sync - scan the workspace directory and `.code-workspace`
  for repos/folders not in the TOML and add them. Only supports entries that
  are direct children of the workspace root (no nested paths). Prompts for
  confirmation before modifying the TOML.
- Filtering flags apply (see §6.13)

---

### 6.4 `ergo status`

Show the state of all repos in a workspace.

```
ergo status [workspace-name]
```

**Output (TUI table):**

```
┌─────────────────────────┬──────────┬────────┬─────────┬───────────────┐
│ Repo                    │ Branch   │ Status │ Behind  │ Group         │
├─────────────────────────┼──────────┼────────┼─────────┼───────────────┤
│ handwriting-recognition │ main     │ clean  │ -       │ ml            │
│ ergo                    │ feat/tui │ dirty  │ 3       │ tools         │
│ kb-core                 │ main     │ clean  │ 1       │ documentation │
│ kb-python               │ main     │ clean  │ -       │ documentation │
└─────────────────────────┴──────────┴────────┴─────────┴───────────────┘
```

**Status values:**
- `clean` - no uncommitted changes
- `dirty` - uncommitted changes present
- `uncloned` - defined in TOML but not yet on disk

**Outside a workspace (in a standalone repo):**
Shows status for just that one repo.

**Flags:**
- `--short` / `-s`: One-line-per-repo, no table borders (for scripting)
- Filtering flags apply (see §6.13)

---

### 6.5 `ergo add`

Add a repo or folder to the workspace TOML.

```
ergo add [workspace-name]
```

**TUI flow (default):**
1. Select: repo or folder?
2. If repo:
   - Prompt for URL
   - Prompt for name (default: derived, Enter to skip)
   - Prompt for branch (default: `main`, Enter to skip)
   - Prompt for tags (comma-separated, optional)
   - Prompt for group (optional)
   - If derived/provided name collides with existing entry → warn and prompt
     for a disambiguating name
3. If folder:
   - Prompt for name
   - Prompt for git initialized? (default: No)
4. Write updated TOML
5. Optionally run `ergo sync` to materialize immediately

**Shorthand (non-interactive):**
```bash
ergo add repo https://github.com/juan/new-project.git --tags=rust,game --group=creative
ergo add repo https://github.com/juan/utils.git --name=utils-personal
ergo add folder design-docs
ergo add folder notes --git
```

**Collision handling:** If a repo with the same derived name already exists in the
workspace, ergo warns and prompts the user to either provide a disambiguating `name`
or cancel.

---

### 6.6 `ergo remove`

Remove a repo or folder from the workspace TOML.

```
ergo remove [workspace-name]
```

**TUI flow (default):**
1. Show list of repos and folders
2. Select items to remove (multi-select)
3. Confirm removal from TOML
4. Files remain on disk (safe by default)

**Shorthand:**
```bash
ergo remove repo old-experiment
ergo remove folder scratch
```

**Flags:**
- `--force`: Also delete the directory from disk. Requires confirmation.

---

### 6.7 `ergo run`

Execute a command across all (or filtered) repos in the workspace.

```
ergo run [workspace-name] -- <command>
```

**Behavior:**
1. Resolve workspace (or operate on single repo if outside a workspace)
2. Filter repos by flags (see §6.13)
3. Exclude repos in groups listed in `[run].excluded_groups` (global config)
4. For each remaining repo:
   - `cd` into the directory
   - Execute `<command>` via shell
   - Capture stdout/stderr
5. Display results (sequentially by default, buffered per-repo if parallel)

**Output format:**
```
━━━ handwriting-recognition ━━━
On branch main
nothing to commit, working tree clean

━━━ ergo ━━━
On branch feat/tui
Changes not staged for commit:
  modified:   cmd/root.go
```

**Group exclusion:** Repos in groups listed in `[run].excluded_groups` are
automatically skipped. This is how you prevent `ergo run -- go test ./...` from
running against documentation or tools repos that don't have Go code.

**Flags:**
- `--fail-fast`: Stop on first non-zero exit code
- `--include-folders`: Also run in non-repo `[[folders]]` (default: folders excluded)
- `--include-group=<group>`: Override exclusion for a specific group
- `--all`: Include all repos regardless of excluded_groups
- Filtering flags apply (see §6.13)

**Examples:**
```bash
# Git status across all repos (excluding documentation/tools groups)
ergo run -- git status

# Run tests in Go repos only
ergo run --tags=go -- go test ./...

# Pull all repos including normally-excluded documentation repos
ergo run --all -- git pull

# Run only in the ml group
ergo run --group=ml -- python -m pytest

# Run in a specific repo
ergo run --name=ergo -- go build ./...

# Outside a workspace, in a regular repo
ergo run -- go test ./...
# → runs in the current repo only
```

---

### 6.8 `ergo list`

List all configured workspaces.

```
ergo list
```

**Output:**
```
┌───────────────────┬───────┬────────────┐
│ Workspace         │ Repos │ Status     │
├───────────────────┼───────┼────────────┤
│ ml-projects       │ 6     │ synced     │
│ startup           │ 4     │ synced     │
│ side-projects     │ 8     │ not synced │
│ knowledge-base    │ 9     │ synced     │
└───────────────────┴───────┴────────────┘
```

**Status values:**
- `synced` - workspace directory exists on disk
- `not synced` - TOML exists but workspace hasn't been materialized yet

---

### 6.9 `ergo validate`

Validate a workspace TOML configuration.

```
ergo validate [workspace-name]
```

**Checks:**
- TOML syntax is valid
- All `[[repos]]` have a `url` field
- No duplicate derived/explicit names within the workspace
- Branch names are non-empty if specified
- Tags and groups are non-empty strings if specified
- `[[folders]]` names don't collide with repo names
- Global config is valid (if checking global)

**Output on success:**
```
✓ ml-projects.toml is valid (4 repos, 2 folders)
```

**Output on failure:**
```
✗ ml-projects.toml has 2 issues:
  Line 12: repos[2] and repos[4] both derive name "utils" - add explicit name to disambiguate
  Line 18: folders[0] name "ergo" collides with repos[1]
```

**Flags:**
- `--all`: Validate all workspace TOMLs in `~/.ergo/workspaces/`

---

### 6.10 `ergo show`

Filter the workspace view by regenerating `.code-workspace` with only matching repos.

```
ergo show <group|tag|all> [workspace-name]
```

**Behavior:**
1. `ergo show <group-name>` - regenerate `.code-workspace` with only repos matching that group,
   plus all folders. Records the filter in the `"ergo"` object.
2. `ergo show --tag=<tag>` - same but filter by tag.
3. `ergo show all` - regenerate `.code-workspace` with everything. Clears any active filter.
4. `ergo show` (no argument) - TUI to select groups/tags to show.

**The root folder is always included regardless of filter.**

This is useful for focusing Copilot's context. When you're working on the ML repos,
`ergo show ml` strips everything else out of VS Code's view so Copilot doesn't waste
context on irrelevant repos.

**This modifies only the `.code-workspace` file, not the TOML or the filesystem.**

---

### 6.11 `ergo edit`

Open the workspace TOML in VS Code.

```
ergo edit [workspace-name]
```

**Behavior:**
1. Resolve workspace
2. Run `code ~/.ergo/workspaces/<workspace-name>.toml`

---

### 6.12 `ergo version` / `ergo update`

```
ergo --version
```
Outputs version to stdout: `ergo v0.1.0`

```
ergo update
```
Checks for a new version and updates if available. Implementation depends on
distribution method (see §9). For a private repo, this could check GitHub releases
via `gh release list --repo juan/ergo` and download the latest binary.

---

### 6.13 Filtering (Global Flags)

Filtering flags are available on any command that operates across repos:
`sync`, `status`, `run`, `show`.

```
--name=<pattern>     # Filter by repo name. Supports glob: --name=*-service
--group=<group>      # Filter by group
--group              # (no value) Launch TUI to select group(s)
--tags=<t1,t2>       # Filter by tags (repos matching ANY of the listed tags)
```

**Combining filters:** Filters are AND-ed. `--group=ml --tags=python` means
repos that are in group `ml` AND have the tag `python`.

**Interaction with excluded_groups:** Filtering flags override `[run].excluded_groups`.
If you explicitly say `--group=documentation`, that group is included even if it's
in the exclusion list. Only the implicit "run on everything" path uses the exclusion.

---

## 7. Architecture

### Package Structure

```
ergo/
├── cmd/                        # Cobra command definitions
│   ├── root.go                 # Root command, --version, global flags
│   ├── init.go
│   ├── sync.go
│   ├── open.go
│   ├── status.go
│   ├── add.go
│   ├── remove.go
│   ├── run.go
│   ├── list.go
│   ├── show.go
│   ├── validate.go
│   ├── edit.go
│   └── update.go
├── internal/
│   ├── config/                 # TOML parsing and validation
│   │   ├── global.go           # Global config (~/.ergo/config.toml)
│   │   ├── workspace.go        # Workspace config parsing + name derivation
│   │   ├── validate.go         # TOML validation logic
│   │   └── types.go            # Config structs
│   ├── workspace/              # Workspace operations
│   │   ├── detect.go           # Workspace detection from CWD
│   │   ├── resolve.go          # Workspace resolution (name → config)
│   │   ├── manager.go          # Clone, pull, sync logic
│   │   ├── filter.go           # Tag/group/name filtering + exclusion
│   │   ├── runner.go           # Cross-repo command execution
│   │   └── status.go           # Repo state inspection
│   ├── vscode/                 # VS Code integration
│   │   ├── generate.go         # .code-workspace file generation
│   │   └── diff.go             # Smart regeneration (compare before write)
│   ├── git/                    # Git operations (shells out)
│   │   └── git.go              # Clone, pull, status, fetch, init
│   ├── github/                 # GitHub operations (shells out to gh)
│   │   └── github.go           # Repo creation, release checking
│   └── tui/                    # Bubble Tea components
│       ├── app.go              # Main TUI application model
│       ├── workspace_select.go # Workspace selection list (with fuzzy filter)
│       ├── repo_table.go       # Status table view
│       ├── init_wizard.go      # Init guided flow
│       ├── add_form.go         # Add repo/folder form
│       ├── remove_select.go    # Multi-select removal
│       ├── group_select.go     # Group/tag selection for show/filter
│       ├── run_output.go       # Command output viewer
│       └── styles.go           # Lipgloss styles
├── TENETS.md
├── go.mod
├── go.sum
├── main.go
└── README.md
```

### Key Dependencies

| Package                              | Purpose                                        |
| ------------------------------------ | ---------------------------------------------- |
| `github.com/spf13/cobra`             | CLI command framework                          |
| `github.com/charmbracelet/bubbletea` | TUI framework                                  |
| `github.com/charmbracelet/lipgloss`  | TUI styling                                    |
| `github.com/charmbracelet/bubbles`   | TUI components (tables, text inputs, spinners) |
| `github.com/BurntSushi/toml`         | TOML parsing                                   |
| `github.com/gobwas/glob`             | Glob pattern matching for `--name`             |

### Git Operations

All git operations shell out to `git` on PATH.

```go
// internal/git/git.go

func Clone(url, dest, branch string) error {
    args := []string{"clone", url, dest}
    if branch != "" {
        args = append(args, "--branch", branch)
    }
    return exec.Command("git", args...).Run()
}

func Pull(repoPath string) error {
    cmd := exec.Command("git", "pull")
    cmd.Dir = repoPath
    return cmd.Run()
}

func Init(path string) error {
    cmd := exec.Command("git", "init")
    cmd.Dir = path
    return cmd.Run()
}

func Status(repoPath string) (RepoState, error) {
    // git status --porcelain for dirty check
    // git rev-list --count HEAD..@{u} for behind count
}
```

### Name Derivation

```go
// internal/config/workspace.go

func DeriveRepoName(url string) string {
    // "https://github.com/juan/handwriting-recognition.git"
    //   → "handwriting-recognition"
    // "git@github.com:juan/my-tool.git"
    //   → "my-tool"
    base := path.Base(url)
    return strings.TrimSuffix(base, ".git")
}
```

### Smart Regeneration

```go
// internal/vscode/diff.go

func WriteIfChanged(path string, content []byte) (changed bool, err error) {
    existing, err := os.ReadFile(path)
    if err == nil && bytes.Equal(existing, content) {
        return false, nil // No change needed
    }
    return true, os.WriteFile(path, content, 0644)
}
```

---

## 8. State Management

### Cached State (`~/.ergo/state/<workspace>.json`)

Lightweight cache to avoid redundant git operations:

```json
{
  "workspace": "ml-projects",
  "last_sync": "2026-04-30T10:30:00Z",
  "repos": {
    "handwriting-recognition": {
      "last_pulled": "2026-04-30T10:30:00Z",
      "commit_hash": "abc123f"
    }
  }
}
```

This is a performance optimization, not a source of truth. If the cache is
missing or corrupt, ergo just does a full sync. No user-facing commands
interact with this file directly.

---

## 9. Error Handling

| Scenario                                    | Behavior                                            |
| ------------------------------------------- | --------------------------------------------------- |
| `git` not on PATH                           | Error: "git is required"                            |
| `gh` not on PATH                            | Error: "gh is required"                             |
| `code` not on PATH                          | Error: "code CLI is required"                       |
| Clone fails (auth, network)                 | Log error, continue with other repos, report at end |
| TOML parse error                            | Error with line number and context                  |
| Workspace dir already exists (`open`)       | Adopt - sync against it                             |
| Repo dir exists but isn't the expected repo | Warn, skip, report                                  |
| Duplicate derived repo names                | Error during validate, prompt for explicit names    |
| No workspace match (partial name)           | TUI with filtered results                           |
| No workspace match at all                   | Error: "No workspace matching '<input>'"            |
| Orphaned directory on disk                  | Warn during sync, never delete without `--force`    |

---

## 10. Future Extensions (v2+ Design Considerations)

These are explicitly out of v1 scope but the architecture should not block them.

### 10.1 Doc Cache Generation

A command like `ergo docs generate` that runs a documentation extraction pass
over repos and stores the output in a known location.

**How the architecture supports it:** Each `[[repos]]` entry could gain a
`[repos.docs]` table with doc generation config. The generated docs land in
a folder within the workspace root. The `.code-workspace` file could optionally
include or exclude the docs folder - `ergo show` already supports dynamic
regeneration of the folder list, so toggling doc visibility fits naturally.

### 10.2 Terminal Agent Integration

`ergo agent` that launches a terminal agent (Claude Code, aider, etc.)
with the workspace context pre-loaded. The agent would receive:
- List of repos and their purposes
- Workspace-level instructions
- Current state (branches, dirty files)

The `"ergo"` object in `.code-workspace` makes it possible for any tool to
discover which workspace it's operating in and read the corresponding TOML.

### 10.3 Task Routing

Configuration in the TOML for routing tasks to different models:
```toml
[workspace.routing]
local_model = "qwen2.5-coder:32b"
cloud_model = "sonnet-4.6"

[workspace.routing.rules]
autocomplete = "local"
refactor = "local"
architecture = "cloud"
code_review = "cloud"
```

---

## 11. Build & Distribution

### Build

```bash
# Single target: macOS arm64
GOOS=darwin GOARCH=arm64 go build -o ergo .
```

### Install (from source, private repo)

```bash
git clone git@github.com:juan/ergo.git
cd ergo
go build -o ergo .
mv ergo /usr/local/bin/  # or ~/go/bin/
```

### Version Embedding

Use Go ldflags to embed version at build time:

```bash
go build -ldflags "-X main.version=0.1.0" -o ergo .
```

### Update Mechanism

`ergo update` checks GitHub releases on the private repo via `gh`:

```bash
# Internally:
gh release list --repo juan/ergo --limit 1
# Compare with current version
# If newer, download:
gh release download <tag> --repo juan/ergo --pattern "ergo-darwin-arm64"
# Replace binary
```

### Future: Homebrew Tap (when open-sourced)

```bash
brew install juan/tap/ergo
```

---

## 12. Example Session

```bash
# Create a new workspace (guided TUI)
$ ergo init ml-projects
# TUI: prompts for repos, folders, writes TOML
Created workspace config: ~/.ergo/workspaces/ml-projects.toml

# Workspace exists in list but isn't materialized yet
$ ergo list
┌───────────────┬───────┬────────────┐
│ Workspace     │ Repos │ Status     │
├───────────────┼───────┼────────────┤
│ ml-projects   │ 4     │ not synced │
└───────────────┴───────┴────────────┘

# Open materializes the workspace (clones, creates folders, opens VS Code)
$ ergo open ml-projects
Cloning handwriting-recognition... done
Cloning ergo... done
Cloning kb-core... done
Cloning kb-python... done
Creating scratch/... done
Creating planning/... (git init) done
Generated ml-projects.code-workspace
Opening in VS Code...

# Add a repo later
$ ergo add repo https://github.com/juan/task-pad.git --tags=hardware --group=ml
Added to ml-projects.toml

# Sync to clone the new repo
$ ergo sync
Cloning task-pad... done
Updated ml-projects.code-workspace
Sync complete: 1 cloned, 4 up to date, 2 folders ok

# Check status
$ ergo status
┌─────────────────────────┬──────────┬────────┬────────┬───────────────┐
│ Repo                    │ Branch   │ Status │ Behind │ Group         │
├─────────────────────────┼──────────┼────────┼────────┼───────────────┤
│ handwriting-recognition │ main     │ clean  │ -      │ ml            │
│ ergo                    │ feat/tui │ dirty  │ 3      │ tools         │
│ kb-core                 │ main     │ clean  │ 1      │ documentation │
│ kb-python               │ main     │ clean  │ -      │ documentation │
│ task-pad                │ main     │ clean  │ -      │ ml            │
└─────────────────────────┴──────────┴────────┴────────┴───────────────┘

# Run tests across only ml repos (documentation group is excluded by default)
$ ergo run --group=ml -- python -m pytest
━━━ handwriting-recognition ━━━
4 passed in 1.23s

━━━ task-pad ━━━
7 passed in 0.89s

# Focus VS Code on just the ml repos
$ ergo show ml
Regenerated ml-projects.code-workspace (filtered: group=ml)

# Back to seeing everything
$ ergo show all
Regenerated ml-projects.code-workspace (all repos)

# Edit the workspace config directly
$ ergo edit
# Opens ~/.ergo/workspaces/ml-projects.toml in VS Code

# Validate after manual edits
$ ergo validate
✓ ml-projects.toml is valid (5 repos, 2 folders)

# Check version
$ ergo --version
ergo v0.1.0

# Update
$ ergo update
Current: v0.1.0
Latest:  v0.2.0
Downloading ergo-darwin-arm64... done
Updated to v0.2.0
```