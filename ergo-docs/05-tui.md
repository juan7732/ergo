# TUI Layer

All Bubble Tea models live in [`internal/tui/`](../../ergo/internal/tui/).
Each model implements the `tea.Model` interface (`Init`, `Update`, `View`)
and exposes a `Result()` method that callers in `cmd/` invoke after the program
exits to retrieve typed output.

## Runtime helpers ([`app.go`](../../ergo/internal/tui/app.go))

```go
Run(m tea.Model) (tea.Model, error)        // alt-screen
RunInline(m tea.Model) (tea.Model, error)  // no alt screen, prints inline
```

Almost every command uses `RunInline` so the TUI feels like a short interactive
prompt that gives the terminal back when done.

## Styles ([`styles.go`](../../ergo/internal/tui/styles.go))

A small lipgloss palette keeps the visual language consistent:

| Color name | ANSI 256     | Used for                            |
| ---------- | ------------ | ----------------------------------- |
| Accent     | 212 (pink)   | titles, cursors, prompt characters  |
| Subtle     | 241 (gray)   | helper text, table borders, hints   |
| Error      | 196 (red)    | error messages, removed-item check  |
| Success    | 82  (green)  | ok/synced, selected toggle in show  |
| Warning    | 214 (orange) | (defined, currently unused in code) |

Composed styles: `StyleTitle`, `StyleSubtle`, `StyleError`, `StyleSuccess`,
`StyleWarning`, `StyleLabel`, `StylePrompt`, `StyleKeybinding`, `StyleSelected`,
`StyleTableHeader`, `StyleTableBorder`.

`KeybindingHint(key, desc)` formats a `<key> <desc>` pair for the help bar at
the bottom of every view.

## Models

### `InitWizard` ([`init_wizard.go`](../../ergo/internal/tui/init_wizard.go))

Multi-step prompt for `ergo init`. Steps tracked in a `wizardStep` enum:

```
stepWorkspaceName → (loop) stepRepoURL → stepRepoName → stepRepoBranch
                                       → stepRepoTags → stepRepoGroup
                  → (loop) stepFolderName → stepFolderGit
                  → stepConfirm → stepDone
```

- Pressing Enter on a blank URL/folder name advances to the next loop or the
  confirm screen.
- `Result()` returns `(WizardResult, confirmed bool)`. `cmd/init.go` calls
  `WriteWorkspace` only when `confirmed == true`.

### `AddForm` ([`add_form.go`](../../ergo/internal/tui/add_form.go))

Used by `ergo add` (no subcommand) for either a repo or a folder.
Inline collision warning (`m.collision`) is shown above the input when the
typed name conflicts with existing entries; clears as soon as the user types.

### `RemoveSelect` ([`remove_select.go`](../../ergo/internal/tui/remove_select.go))

Multi-select list using ↑/↓ + space + enter. Items are `RemoveItem{Name, IsRepo}`.
Returns the slice of selected items.

### `GroupSelect` ([`group_select.go`](../../ergo/internal/tui/group_select.go))

Used by `ergo show` when invoked with no positional arg or `--tag` flag.
Sectioned: groups first, then tags. Cursor + space + enter.

`Result()` rule: if any groups are selected, the **first selected group** wins
and tags are dropped (mirrors the positional grammar of `ergo show <group>`).
Otherwise returns the selected tags.

### `WorkspaceSelect` ([`workspace_select.go`](../../ergo/internal/tui/workspace_select.go))

Filterable workspace picker. Pre-fills the filter with the user's typed
partial name. Real-time substring filtering as the user types. Used by
`resolveWorkspaceName` whenever resolution returns multiple candidates.

### `SearchSelect` ([`search_select.go`](../../ergo/internal/tui/search_select.go))

Live-filter picker over the full search index, used by `ergo search` with no
query. Built on the bubbles `list` component rather than a hand-rolled
cursor: the list starts in its filtering state so the first keystroke narrows
it (fzf-style), items paginate past 12 rows, and each item's `FilterValue()`
is name + URL + workspace, the same fields the CLI query matches. The default
fuzzy filter is a deliberate divergence from the CLI's substring match (a
`// DECISION:` in source).

Enter, Esc/Ctrl-C, and Up/Down are intercepted before the list sees them,
because in filtering state the list would treat Esc as "clear filter" and
Enter as "apply filter". `q` is a filter character, not a cancel key.
`Result()` returns `(workspace.Hit, bool)`. The caller runs the program with
`tea.WithOutput(os.Stderr)` so stdout stays free for the selected path.

`HitLine` and `HitStateLabel` are exported rendering helpers shared with the
`ergo search` table so the state vocabulary (`cloned`, `created`, `synced`
and their absent forms) has one definition.

Covered by `teatest` unit tests in `search_select_test.go` (typing narrows,
Enter yields the hit, Esc and Ctrl-C cancel).

### `RenderRepoTable` / `ShortRepoLine` ([`repo_table.go`](../../ergo/internal/tui/repo_table.go))

Pure rendering helpers (not Bubble Tea models). Used by `cmd/status.go` for
both interactive and `--short` output.

`statusValues(entry)` derives the display strings:

| Condition  | status     | branch | behind        |
| ---------- | ---------- | ------ | ------------- |
| `Uncloned` | `uncloned` | `—`    | `—`           |
| `Dirty`    | `dirty`    | actual | actual or `—` |
| otherwise  | `clean`    | actual | actual or `—` |

`ShortRepoLine` is tab-separated for parsing in shell pipelines.

### `PrintRunResult` ([`run_output.go`](../../ergo/internal/tui/run_output.go))

Prints one run result with a header bar:

```
━━━ <repo-name> ━━━
<combined stdout+stderr>
exit code N    # only when non-zero
```

Followed by a blank line. Called per-result from `RunAcrossTargets`'s
`OnResult` callback so output appears as repos finish (in completion order
when running in parallel).

## Conventions

- **Keys**: Enter confirms, Esc/Ctrl-C cancels, q quits where appropriate.
  Every view shows a help bar at the bottom via `KeybindingHint`.
- **Cancellation**: every model has `cancelled bool` set on Esc/Ctrl-C; `Result()`
  returns `false` for the second tuple value to signal the cancel.
- **No alt screen for short flows.** All wizards/selectors use `RunInline` so
  output is preserved in scrollback after the TUI exits.
- **No spinners.** Operations either complete immediately (TOML edits) or print
  per-line progress (sync/run). The implementation plan calls out spinners as
  a future polish but they are not used in the current code.
