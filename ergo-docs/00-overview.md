# ergo — Overview

> Comprehensive reference documentation for the `ergo` CLI tool.
> Sourced directly from the codebase at [`ergo/`](../../ergo/) on May 1, 2026.

## What ergo Is

`ergo` is a single-binary Go CLI for managing **multi-repo VS Code workspaces**.
It treats a TOML file in `~/.ergo/workspaces/<name>.toml` as the **single source of
truth** for what makes up a workspace; the on-disk directory layout and the
generated `.code-workspace` file are derived artifacts that ergo can rebuild at
any time.

The product motivation is to give AI-assisted development tools (Copilot,
Claude, terminal agents, local LLMs) **structured, deep multi-repo context**.
Pulling related repos into one VS Code workspace gives the assistant visibility
into upstream/downstream code; `ergo` automates the bookkeeping required to keep
that workspace consistent.

## At-a-Glance

| Property                      | Value                                                              |
| ----------------------------- | ------------------------------------------------------------------ |
| Language                      | Go (module `github.com/juan7732/ergo`, go `1.26.1`)                |
| CLI framework                 | `spf13/cobra`                                                      |
| TUI framework                 | `charmbracelet/bubbletea` + `bubbles` + `lipgloss`                 |
| TOML parser                   | `BurntSushi/toml`                                                  |
| Glob matching                 | `gobwas/glob`                                                      |
| Concurrency                   | `golang.org/x/sync/errgroup` + bounded semaphore                   |
| Tests                         | `stretchr/testify` (assert + require)                              |
| Build target                  | macOS arm64 (`GOOS=darwin GOARCH=arm64`)                           |
| Shell dependencies            | `git`, `gh`, `code` on `$PATH`                                     |
| Single source of truth        | TOML                                                               |
| Distribution                  | `go install` from source, or GitHub release binary + `ergo update` |
| Self-update repo (hard-coded) | `juan7732/ergo`                                                    |

## File Map

The repository lives at [ergo/](../../ergo/). High-level layout:

```
ergo/
├── main.go                         # entry point: passes embedded version into cmd.Execute
├── go.mod / go.sum
├── justfile                        # task runner (build, test, lint, release)
├── README.md
├── .github/
│   ├── copilot-instructions.md     # AI assistant instructions
│   ├── skills/commit-push/SKILL.md # commit-push automation
│   └── workflows/                  # CI + release pipelines
├── cmd/                            # one file per cobra command
│   ├── root.go            add.go          edit.go         helpers.go
│   ├── init.go            list.go         open.go         remove.go
│   ├── run.go             show.go         status.go       sync.go
│   ├── update.go          validate.go
├── internal/
│   ├── config/                     # global + workspace TOML parsing/validation
│   ├── git/                        # thin wrapper around `git` CLI
│   ├── github/                     # thin wrapper around `gh` (used only by update)
│   ├── vscode/                     # .code-workspace generation + smart write
│   ├── workspace/                  # detect, resolve, sync, run, status, state, filter
│   └── tui/                        # all Bubble Tea models + lipgloss styles
├── test/integration/               # dockerized end-to-end harness
└── bin/ergo                        # build artifact
```

The companion plan and specification live one folder up:

- [tenets.md](./tenets.md) — design philosophy.

## Document Index

This documentation set is organized into focused topic files:

1. [01-architecture.md](01-architecture.md) — package layout, dependency graph, key types.
2. [02-configuration.md](02-configuration.md) — TOML formats, defaults, validation rules.
3. [03-commands.md](03-commands.md) — every command, flag, and exit behavior.
4. [04-internals.md](04-internals.md) — package-by-package internals tour.
5. [05-tui.md](05-tui.md) — Bubble Tea models and styling.
6. [06-workflows.md](06-workflows.md) — typical user flows end-to-end.
7. [07-operational-semantics.md](07-operational-semantics.md) — error model, exit codes, idempotency, parallelism, state cache.
8. [08-build-test-release.md](08-build-test-release.md) — justfile, CI, release pipeline, integration suite.
9. [09-design-tenets.md](09-design-tenets.md) — distilled tenets that constrain every feature.
10. [10-extension-points.md](10-extension-points.md) — v2 candidates and where they would slot in.
