---
name: commit-push
description: 'Commit and push changes in the ergo repo. Use when the user asks to commit, push, ship, or land changes. Runs the full pre-commit pipeline (tidy, fmt, vet, test-race), reviews staged diff, writes a conventional commit, and pushes to the current branch. Refuses to push to main without confirmation. For cutting a versioned release, use cut-release instead.'
argument-hint: '[optional commit message]'
---

# Commit & Push

End-to-end commit workflow that mirrors CI. Stop and report on any failure;
never bypass checks (no `--no-verify`, no `--force`).

## When to Use

- "commit and push", "ship it", "land this", "publish these changes"
- The user is done with a unit of work and wants it on the remote

## Pre-flight

Run these in parallel before touching anything:

```bash
git status --short
git rev-parse --abbrev-ref HEAD
```

- If branch is `main` or `master`: **stop** and ask the user to confirm pushing
  directly. Suggest a feature branch.
- If working tree is empty: report nothing to commit and stop.

## Pipeline

Run the local equivalent of CI. Stop on first failure and surface the error.

1. **Tidy modules** — may modify `go.mod`/`go.sum`
   ```bash
   just tidy
   ```

2. **Check** — runs `fmt`, `vet`, and `test-race` together
   ```bash
   just check
   ```

   If `just check` fails:
   - Formatting rewrote files: re-stage and continue.
   - Vet/test failures: stop, report, do not proceed.

## Stage & Review

3. **Stage everything**
   ```bash
   git add -A
   ```

4. **Show what's staged** before committing
   ```bash
   git diff --cached --stat
   ```

   For non-trivial changes, also show the full diff:
   ```bash
   git diff --cached
   ```

## Commit

5. **Compose the message** if the user didn't provide one. Use
   [conventional commits](https://www.conventionalcommits.org/):

   ```
   type(scope): short imperative summary

   Optional body explaining the why.
   ```

   - `type`: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `build`, `ci`
   - `scope`: package or area touched (e.g., `config`, `tui`, `git`)
   - Summary: lowercase, no trailing period, imperative mood

6. **Commit**
   ```bash
   git commit -m "<message>"
   ```

## Push

7. **Push to the current branch**
   ```bash
   git push origin HEAD
   ```

   If the branch has no upstream, set it:
   ```bash
   git push -u origin HEAD
   ```

## Hard Rules

- Never `git push --force` or `--force-with-lease` unless explicitly asked.
- Never use `--no-verify` to skip hooks.
- Never `git reset --hard`, `git clean -fd`, or discard untracked files to "clean up".
- Never amend a pushed commit without explicit confirmation.
- If on `main`/`master`, require explicit user confirmation before pushing.

## On Failure

Report the failing step with its exit output verbatim. Do not retry blindly.

| Failure | Action |
|---|---|
| `just tidy` changed files | Continue; they'll be staged in step 3. |
| `just check` formatting | Continue; fmt rewrote files. |
| `vet` errors | Stop. Fix the reported issues. |
| Test failures | Stop. Investigate the failing test. |
| Push rejected (non-fast-forward) | Stop. Ask the user how to reconcile (rebase/merge). |
