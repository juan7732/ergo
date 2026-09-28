# ergo privacy policy

*Applies to the `ergo` command-line tool published from
https://github.com/juan7732/ergo, including the `juan7732.ergo` winget package
and the `juan7732/tap/ergo` Homebrew formula. Last updated: 2026-09-28.*

ergo is a local command-line tool. It runs no service, has no account system,
and collects no telemetry. This document describes exactly what it reads,
writes, and sends over the network so you can judge that for yourself.

## Summary

- ergo does **not** collect, transmit, or store analytics, usage data, crash
  reports, or any personal information.
- ergo does **not** read, store, or transmit credentials. Authentication to
  git hosts and GitHub is handled entirely by the `git`, `ssh`, and `gh`
  tools already on your machine, using their own credential stores.
- The only network traffic ergo causes is the `git clone` / `git pull` it
  runs against repositories **you** listed in your workspace file, and the
  GitHub release download performed by `ergo update` when **you** run it.
- Everything ergo writes lives in your home directory and your workspace
  directory. Deleting those removes all of it.

## What ergo stores on your machine

| Path | Contents | Written by |
| --- | --- | --- |
| `~/.ergo/config.toml` | Global defaults: workspace root, default branch, parallelism, auto-pull, excluded groups, git protocol (`https` or `ssh`) | You, or `ergo edit --global` |
| `~/.ergo/workspaces/<name>.toml` | Your workspace declaration: repository URLs, names, branches, tags, groups, optional VS Code settings, folder names | `ergo init`, `ergo add`, `ergo remove`, or you |
| `~/.ergo/state/<name>.json` | Optional cache: last sync time and per-repository last-pull timestamp. Safe to delete at any time. | `ergo sync` |
| `<workspace_root>/<name>/<name>.code-workspace` | VS Code workspace file: folder paths and an `ergo` object with the workspace name and active view filter | `ergo sync`, `ergo open`, `ergo show` |
| `<workspace_root>/<name>/<repo>/` | The repositories and folders you declared, cloned with `git` | `ergo sync` |

ergo never stores commit contents, diffs, file lists, usernames, email
addresses, tokens, or SSH keys. Files it creates under `~/.ergo/state` are
written with owner-only permissions.

## What leaves your machine, and to whom

**Git hosts you configure.** `ergo sync` (and the sync prompt after
`ergo add`/`ergo init`) runs `git clone` for repositories not yet on disk and
`git pull --ff-only` for those already present when auto-pull is enabled.
Those commands talk to the remotes in your workspace file using your own git
configuration. What is sent is what `git` sends: the fetch request and, for
authenticated remotes, whatever credentials your git credential helper or
ssh-agent supplies. ergo does not see or handle those credentials. If
`[git].protocol = "ssh"` is set, ergo rewrites `https://` URLs to SSH form in
memory at clone time only; your workspace file is never modified.

**GitHub, when you run `ergo update`.** `ergo update` shells out to the GitHub
CLI (`gh`) to list the latest release of `juan7732/ergo`, download the binary
for your platform plus `checksums.txt`, verify the SHA-256, and replace the
running binary. It runs only when you invoke it; ergo never checks for
updates on its own. The request is made by `gh` with the authentication `gh`
already holds, and is subject to GitHub's privacy statement. A binary
installed by Homebrew refuses to self-update and directs you to `brew
upgrade`.

**Nothing else.** Read-only commands (`status`, `list`, `show`, `validate`,
`config`, `search`) do not touch the network. `ergo run -- <command>`
executes the command you supply in each repository; any network activity is
that command's, not ergo's. `ergo open` and `ergo edit` launch VS Code
locally. The ergo binary contains no HTTP client of its own.

## Credentials

ergo has no credential handling code. It does not prompt for, read, cache, or
transmit passwords, tokens, or keys. All authentication is delegated to
`git`, `ssh`, and `gh` and governed by their configuration.

## Telemetry and analytics

None. ergo does not send usage data, error reports, or identifiers anywhere,
and has no opt-in for doing so.

## Retention and deletion

All data is kept on your disk for as long as you keep it. To remove
everything ergo has written:

1. Delete the workspace directories under your workspace root (or run
   `ergo sync --force`, which deletes only directories no longer declared,
   after an explicit confirmation).
2. Delete `~/.ergo/`.

There is no server-side data to request or delete.

## Children

ergo is a developer tool and is not directed at children.

## Changes

Changes to this policy are made by commit to this file; the history is at
https://github.com/juan7732/ergo/commits/main/PRIVACY.md.

## Contact

Open an issue at https://github.com/juan7732/ergo/issues. For security
concerns, see [SECURITY.md](SECURITY.md).
