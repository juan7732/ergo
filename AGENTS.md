# AGENTS.md

Conventions for agents working in the ergo codebase.

## Orientation

- Design tenets and how they map to code: `ergo-docs/09-design-tenets.md`. Read it before changing behavior; tenet violations must be conscious and documented, not accidental.
- Full docs live in `ergo-docs/` (architecture, commands, internals, TUI, operational semantics, build/test/release).

## Decision comment taxonomy

Mark every judgment call the spec or tenets did not force, using this fixed vocabulary:

- `// DECISION:` intentional choice among plausible alternatives, with the reason
- `// REVIEW:` unsure, a human should check this
- `// SPEC:` cites the spec or doc section that forced the code
- `// TODO:` known gap

`rg "// (DECISION|REVIEW):"` must list every judgment call in the codebase. If you make a call the spec is silent on, mark it; if you are not confident, mark it `REVIEW`, not `DECISION`.

## Code conventions

- Abstractions are test seams only: the one-method `git.Runner` interface and package-level function vars. Do not widen an interface until a second real implementation exists.
- Sync and other non-flagged operations never delete; destruction requires an explicit flag plus confirmation.
- Destruction scope is always computed from the full config, never a filtered view.
- Generated files go through `WriteIfChanged`; report updated vs unchanged.
- Integration tests exec the real binary in the hermetic harness (`test/integration/harness/`); stub external tools with recording PATH shims, never mocks of internal packages.
