# Contributing to ergo

Thanks for your interest in contributing! ergo is a small, opinionated tool, and
contributions are welcome — but please read this document first so we don't waste
each other's time.

## Ground rules

- **Be kind.** Assume good intent. No harassment, personal attacks, or hostility.
- **Discuss before building.** For anything non-trivial (new commands, config
  changes, behavior changes, new dependencies), open an issue first. PRs that
  show up unannounced with large changes are likely to be closed.
- **Bug reports are contributions too.** A good reproducer is more valuable than
  a half-finished PR.
- **Scope discipline.** ergo follows a written spec ([SPEC.md](SPEC.md)) and a
  set of design tenets ([TENETS.md](TENETS.md)). Features outside that scope
  will be declined regardless of quality.

## Before you start

1. Read [SPEC.md](SPEC.md) — the behavior contract.
2. Read [TENETS.md](TENETS.md) — the principles that resolve ambiguity.
3. Skim [ergo-docs/](ergo-docs/) for architecture and internals.
4. Search existing issues to avoid duplicates.

## What we welcome

- Bug fixes with a failing test that the fix turns green.
- Documentation improvements (typos, clarifications, examples).
- Test coverage for under-tested code paths.
- Performance fixes with measurements.
- Spec-conformant changes that have been discussed in an issue.

## What we'll likely decline

- Features that aren't in the spec and haven't been discussed.
- New configuration options for behavior the spec says is hardcoded.
- Refactors without a concrete motivating problem.
- New dependencies without strong justification.
- Drive-by style changes mixed into functional PRs.
- Cross-platform support beyond macOS arm64 (current build target).

## Development setup

Requirements:

- Go (version per [go.mod](go.mod))
- [`just`](https://github.com/casey/just) — task runner
- `git`, `gh`, `code` on PATH (for running ergo locally)
- Docker (only for integration tests)

```bash
git clone https://github.com/juan7732/ergo.git
cd ergo
just --list          # see all recipes
just check           # fmt + vet + test-race
```

## Workflow

1. **Fork** the repo and create a topic branch off `main`.
2. **Write a test first** when fixing a bug or changing behavior.
3. **Keep PRs focused.** One logical change per PR.
4. **Run `just check`** before pushing. PRs failing CI won't be reviewed.
5. **Run integration tests** (`just integration`) if your change touches command
   behavior, the workspace lifecycle, or the shell-out boundary.
6. **Write a clear commit message.** Conventional commits encouraged
   (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`).
7. **Open a PR** against `main` with a description that explains *what* and
   *why*, links the related issue, and notes anything reviewers should focus on.

## Code conventions

The full conventions live in [`.github/copilot-instructions.md`](.github/copilot-instructions.md).
Highlights:

- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- User-facing error messages: lowercase, no trailing punctuation, actionable.
- Table-driven tests, `testify/assert` + `testify/require`.
- No `utils` / `helpers` packages.
- Don't introduce interfaces until there's a second implementation
  (shell boundaries excepted).
- Don't add docstrings or comments to code you didn't change.

## Reporting bugs

Open an issue with:

- ergo version (`ergo --version`)
- macOS version
- Minimal reproducer (TOML snippet + commands run)
- What you expected vs. what happened
- Relevant output (redact anything sensitive)

## Reporting security issues

Do **not** open a public issue for security vulnerabilities. Email the
maintainer directly (see commit history for contact) with details and a
reproducer. We'll acknowledge within a reasonable window and coordinate
disclosure.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE) that covers the project.
