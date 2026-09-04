# Roadmap

Where ergo is going. The past is recorded in [release-notes/](release-notes/);
this document owns the future. Items graduate out of here and into release
notes when they ship, and every entry carries a tenet check — anything that
can't justify its cognitive load ([09-design-tenets.md](09-design-tenets.md),
tenet 5) doesn't get listed as committed.

Mechanical v2 candidates with identified code slots (doc cache, `ergo agent`,
task routing) remain catalogued in
[10-extension-points.md](10-extension-points.md); this document is about
direction and sequencing, and links there rather than duplicating.

## Horizons

| Horizon         | Meaning                                                        |
| --------------- | -------------------------------------------------------------- |
| **Now**         | Committed and scoped; next release(s).                         |
| **Next**        | Committed in direction, not yet scoped.                        |
| **Exploring**   | Active research; open questions must close before committing.  |
| **Not planned** | Considered and rejected, with the reasoning kept on record.    |

---

## Theme 1 — Daily use

### Now: non-interactive polish on `ergo add repo`

Found while an agent dogfooded adding repos (2026-08-12): the
non-interactive shorthand `ergo add repo <url> --group=...` still emits the
`sync workspace now? [y/N]` prompt even when stdin is not a TTY (the docs
say the prompt is TTY-gated — verify whether the gate is actually applied on
the shorthand path; this may be a bug, not a decision). It degrades safely
(EOF → N), but agents end up defensively piping `</dev/null`. Fix the gate,
and consider a `--sync` flag so intent fits in one call.

**Tenet check:** optimize for the hands (and for agents' hands); the flag is
one small commitment that removes a prompt round-trip from the most
script-shaped command.

### Next: workspace config sync (the `.ergo` repo)

Solves "which of my three machines has this workspace?" and cross-machine
workspace drift. Direction committed 2026-08-13; design sketch agreed, exact
scoping open.

**Design sketch (mirror semantics):**

- A git repo, default `<gh-user>/.ergo`, holds one directory per device:
  `/{device-words}/<workspace>.toml` plus a small per-device manifest
  (hostname, last-push timestamp). Account defaults to the `gh` authed user;
  created via `gh repo create`, GitHub-only in v1 (consistent with
  `ergo update`; mark `// DECISION:`).
- **Always created private.** ergo never changes visibility; making the repo
  public is the user's own action on GitHub, on their own time.
- **Each device writes only its own directory**, which mirrors that device's
  `~/.ergo/workspaces/`. Same-name workspaces on different devices coexist
  as different files (drift is information, not corruption); local deletion
  converges as removal from the device's own directory only; no shared
  files means no merge conflicts, structurally. A fresh machine gets a
  fresh identity and an empty directory, so it can never mass-delete.
- **Converge on command, no daemon.** ergo keeps a local clone (e.g.
  `~/.ergo/sync/`); any command that loads configs compares and best-effort
  commits/pushes changes. This captures hand edits (most edits never pass
  through ergo: `ergo edit` spawns the editor and exits). Offline
  accumulates commits; push retries on the next invocation; sync never
  blocks or fails the underlying command (state-cache discipline).
- **Cross-device reads are explicit**: a restore verb (working name
  `ergo restore [workspace] [--from <device>]`, TUI picker on ambiguity)
  pulls another device's definition; never silently overwrites a local
  TOML. Not an `init` flag: init authors, restore copies.
- **Device identity**: two random words plus a short suffix (e.g.
  `crimson-otter-3f2a`), generated once at enable time, stored in global
  config. Not hostname-derived (hostnames collide and change).
- **Rides along:** `ergo delete <workspace>`. No deletion verb exists
  today (workspaces are removed by hand-deleting the TOML), and sync makes
  the gap visible. Committed 2026-08-13.
- **Follow-up:** `ergo search` extends across devices by reading the local
  sync clone (offline, no network), adding an additive `device` field to
  the search JSON. Noted in [plans/ergo-search.md](plans/ergo-search.md).
- **corvo boundary (decided 2026-08-13):** replication of ergo's own
  workspace configs is ergo's job; this supersedes corvo's original PF13
  corvo-as-replication-authority model (reconciled in corvo-docs). corvo
  keeps file-level and WIP replication and personal configs, and its
  per-Mac agent is the optional timeliness trigger for ergo's convergence.
  The hub (`corvod`) is not involved: it runs on home infrastructure, not
  on the Macs where ergo runs.

**Open questions before scoping:** final verb naming; `ergo devices`
list/prune for orphaned device directories (reinstalls abandon a
directory); whether global config ever syncs (excluded from v1: it holds
device identity and machine-specific preferences); corvo-agent trigger
mechanics.

**Tenet check:** extends "TOML is truth" across machines (the same
declaration materializes the same system on any machine, now from any
machine); safe by default (explicit pulls, never overwrite, private
always); no daemon, no watcher; best-effort like the state cache.

---

## Theme 2 — Agentic context orchestration

ergo's default description is "VS Code workspace manager," but what it
actually provides is **systems thinking through agentic context
orchestration**: the TOML is a declarative statement of *what belongs in
context together*, groups/tags are context scopes, `ergo show` focuses that
context, `ergo run` fans work out across it, and the `.code-workspace` file
is merely one *renderer* of the model. VS Code becomes one consumer among
several, not the identity.

### Now: positioning reframe

Rewrite the README lead and `00-overview.md` framing around context
orchestration, with VS Code workspace management as the flagship application
of it rather than the definition. Costs nothing in code; sets the direction
publicly. Messaging beats mechanism here — adoption follows a clear story
about *why this exists now*.

**Tenet check:** zero code, zero new commitments.

### Exploring: sandboxed agent runtime on ergo primitives

**Layering statement (decided 2026-08-12):** the harness is built *on* ergo,
not *into* it. ergo's tenets are the tenets of a substrate — neutral,
deterministic, declarative context plumbing — and a harness is opinionated
where ergo is neutral (agent loop, permissions, scheduling, conversation
state). The ecosystem layering:

- **ergo** — the context substrate: what belongs together, materialized
  reproducibly (repos, docs, scopes via groups/tags, fan-out via `run`).
- **[corvo](https://github.com/juan7732/corvo)** — the project context
  provider: why a project exists and where it stands over time (intake,
  memory, lifecycle state, re-entry briefs). Composes with ergo rather than
  absorbing it — per its own design docs. Boundary decision (2026-08-13):
  replication of ergo's own workspace configs belongs to ergo (see the
  config sync item in Theme 1); corvo keeps file-level and WIP replication
  plus personal configs, and its per-Mac agent may trigger ergo's
  convergence.
- **the harness** — a third layer that binds an agent loop to
  ergo-materialized workspaces and corvo-provided memory. Whether it is a
  new project or grows out of `corvo agent`'s dispatch concern is open;
  either way it is not ergo.

The differentiated claim: today's harnesses are repo-scoped (one working
directory). A **systems-thinking-first harness** takes a *system* as its
unit of work — multiple repos + upstream docs + persistent conversation
context, declared in TOML and materialized on demand. ergo is the
load-bearing, already-real layer of that stack, which is why it must stay
substrate-pure and grow substrate-grade guarantees, not a loop.

The research direction: an agent runtime, sandboxed and able to run
autonomously, that uses ergo to assemble its world —

- ergo pulls in the set of repos for a product as a **read-only context
  space** the agent grounds itself in;
- one or more **conversation folders** hold agent-managed, conversation-
  specific context that evolves over time (ergo's `[[folders]]` with
  `git = true` is the obvious seed);
- the agent interacts with the workspace *through the ergo CLI itself* —
  no new protocol surface, because LLMs are already fluent in CLIs.

Open questions to close before anything is committed:

1. **Isolation** — what does ergo need for hermetic operation? Per-invocation
   config/workspace roots (e.g. an `ERGO_HOME` override) so a sandbox doesn't
   share `~/.ergo` with the human operator?
2. **Read-only enforcement** — is "read-only context space" a convention the
   runtime enforces (filesystem permissions, sandbox mounts) or does ergo
   need a notion of it (e.g. a repo attribute that skips push/pull)?
3. **Headless guarantees** — which commands must be certified fully
   non-interactive (no TUI fallback, no confirmation prompts) for unattended
   use? Most already are; this needs an audit, not a rewrite.
4. **What's missing from the JSON surface** — which questions does an agent
   ask that `status/list/config/show/search --json` can't yet answer?

The v2 candidates in [10-extension-points.md](10-extension-points.md)
(doc cache, `ergo agent`) are natural components of this runtime and get
prioritized by whatever the exploration finds, not before.

### Exploring: MCP surface (`ergo mcp`)

*(Promoted from Not planned, 2026-08-12, after weighing where a CLI cannot
reach.)* The principle stands: **CLI-first**. For any harness with shell
access, MCP adds schema overhead for capabilities the shell already
delivers, so an MCP server would only ever be a **thin adapter over the
`--json` contract** — same discipline as ergo-vscode, never reimplementing
logic. That bounds the maintenance cost to "keep ~5 tool descriptions
honest," and the additive-only output contract keeps version-skew risk
structurally low.

Commit when (any one suffices):

1. **A shell-less harness we actually use needs it** — claude.ai-style
   connectors, mobile, IDE chat modes without terminal permission; they can
   consume MCP but cannot exec.
2. **The agent runtime needs interface-level read-only enforcement** — an
   MCP surface exposing only the read verbs (`search`/`status`/`config`/
   `context`) and *not* `sync`/`run` makes "read-only context space" a
   property of the interface rather than of sandbox filesystem policy.
3. **A deliberate registry-driven adoption push** — MCP registries as a
   discoverability channel for agents that don't know ergo exists.

Until one of these is true, the answer remains the CLI. This exploration is
coupled to the agent-runtime exploration above (criterion 2 is its likely
trigger).

---

## Theme 3 — VS Code extension (ergo-vscode)

The extension is the context-control surface *inside* the editor, built as a
thin UI over the ergo binary's `--json` contract.

### Now: M2 (mutations)

As specified in the extension's milestone plan: sync, filter set/clear from
the status bar, TOML watcher, validation diagnostics.

### Exploring: direction beyond M2

Deliberately unscoped until M2 has been dogfooded. Candidates on the table —
Marketplace publication as an adoption funnel, registering ergo operations as
VS Code Language Model Tools so Copilot agent mode can call them, onboarding
walkthroughs — but no commitment until real M2 usage says which of these
earns its complexity.

**Tenet check (for the deferral itself):** "pattern must hurt twice before
getting a feature" applies to roadmap entries too.

---

## Maintenance of this document

- When an item ships, delete it here and record it in the release notes.
- When an exploration closes, either promote its outcome to **Now/Next**
  (with scope) or move it to **Not planned** (with reasoning).
- New entries state the motivation and pass an explicit tenet check.
