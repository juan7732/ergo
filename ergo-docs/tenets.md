# ergo — Design Tenets

> These tenets guide both what ergo is and how it's built. They are not ranked —
> when two tenets pull in different directions, use judgment. But if a decision
> violates a tenet, it should be conscious and documented, not accidental.

---

## Optimize for the hands.

Ergo means work. Ergonomics means fitting the tool to the human. Every command
should feel like the obvious thing to type. If a user checks `--help` more than
once for the same command, the API failed.

This means: short flags for frequent operations. Smart defaults that do what you
probably meant. TUI fallback when a required argument is missing instead of an
error message. Consistent flag names across commands. The happy path should
require the fewest keystrokes.

---

## Earn every abstraction.

Nothing gets built speculatively. A pattern has to hurt at least twice before
it gets a dedicated feature. If you can accomplish something by composing
existing commands, that's not a missing feature — that's the system working.

This is why workspace templates aren't in v1. This is why `ergo run` exists
instead of wrapping every possible git or gh operation. Resist the urge to
preemptively solve problems you haven't had yet.

---

## Every command, flag, and config key is a commitment.

Once it's in the API, someone depends on it — including future you with muscle
memory and aliases built on top. Adding is easy, removing is breaking.

Think before adding. When in doubt, leave it out. A flag you didn't ship is a
flag you never have to support. A config key you didn't add is a config key
that can't confuse anyone.

---

## Small verbs, big compositions.

ergo provides a small set of precise commands. Power comes from combining them
with each other and with the shell. `ergo run --tags=go -- go test ./...` is
three simple ideas composed into something specific and powerful.

Users build their own workflows through aliases and scripts on top of a stable,
minimal surface. The tool doesn't need to anticipate every workflow — it needs
to be a reliable building block for all of them.

---

## Complexity is debt. Pay it only when the interest is worth it.

Every feature, every config option, every flag is complexity debt. The bar for
adding something isn't "would this be useful?" — plenty of things are useful.
The bar is "does this justify the cognitive load it adds to every person who
reads `--help`?"

When complexity is warranted, contain it. A complex feature behind a simple
interface is good engineering. A simple feature with a complex interface is a
design failure.

---

## Speed is respect.

The tool never makes you wait when it doesn't have to. Sync clones in parallel
when asked. Status reads from cache when fresh enough. The TUI renders instantly.
Commands that don't need network access never touch the network.

If an operation takes more than a second, show progress. If it takes more than
five seconds, something is wrong or needs a `--parallel` flag.

Build fast by default. Optimize with intent.

---

## Show your work.

The user always knows what ergo is doing and what it did.

Sync reports what it cloned, pulled, created, and skipped. Run labels output by
repo. Errors include context — what was attempted, what went wrong, and what to
try next. No silent failures. No hidden state changes. No magic.

When ergo makes a decision (like skipping a repo because it's already up to
date), say so. Transparency builds trust; silence breeds suspicion.

---

## TOML is truth.

The configuration file is the single source of truth for a workspace. The
`.code-workspace` file is a derived artifact — generated, never hand-edited.
The filesystem is a reflection of the config, not the other way around.

If the TOML and the filesystem disagree, the TOML wins and sync reconciles.
This means the config is always portable, diffable, reviewable, and version-
controllable. You can reason about a workspace by reading one file.

---

## Safe by default, destructive by intent.

ergo should never cause data loss through normal operation. Running any command
without explicit destructive flags should be safe to do at any time, including
by accident.

**Sync never deletes.** Removing a repo from the TOML doesn't delete it from
disk — it just stops being managed. Sync warns about orphaned directories but
leaves them alone.

**Idempotency is a feature.** Running any command twice produces the same
result. `ergo sync` twice in a row is a no-op the second time. `ergo open`
regenerates the `.code-workspace` to the same content. No surprises.

**Destruction requires intent.** Deleting repos from disk requires `--delete`
and confirmation. `ergo remove` modifies the TOML but asks before touching
the filesystem, defaulting to No.

**Own what you create, respect what you don't.** ergo owns the `.code-workspace`
file (marked with the `ergo-workspace` key) and overwrites it freely — it's
derived. ergo created the workspace directory structure but the contents of
repos and folders belong to the user. The line is clear: ergo owns the
structure, you own the content.

---

## Alias-friendly by design.

The command surface is stable enough that users can build muscle memory and
aliases on top of it.

```bash
alias es="ergo sync"
alias eo="ergo open"
alias er="ergo run --"
alias est="ergo status"
```

Commands compose with shell idioms, not against them. Output is parseable when
piped, pretty when interactive (detect TTY). Exit codes are meaningful. Flags
don't change semantics between commands.

The best ergo workflows are the ones users build themselves.