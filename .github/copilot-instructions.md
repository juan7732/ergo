# ergo — Implementation Instructions

> **For:** AI coding assistants working on this codebase.
> **Read first:** `SPEC.md` (behavior), `TENETS.md` (principles).

---

## Project

ergo is a Go CLI that manages multi-repo VS Code workspaces. TOML config is
the single source of truth. The `.code-workspace` file is a derived artifact.

**Stack:** Go · Cobra · Bubble Tea / Bubbles / Lipgloss · BurntSushi/toml · gobwas/glob
**Shell dependencies:** `git`, `gh`, `code` (assume all on PATH)
**Build target:** macOS arm64

---

## Decision Protocol

1. **Spec is explicit → follow it.** Don't improve or extend.
2. **Spec is silent → check tenets.** They resolve most ambiguity.
3. **Tenets don't resolve it → simplest option that doesn't close doors.** Mark with `// DECISION:`.
4. **Genuine architectural gap → stop and describe it.** Don't guess on anything structurally significant.

---

## Markers

```go
// DECISION: <what you chose and why — spec didn't prescribe>
// REVIEW:   <not confident this is right — please check>
// TODO:     <known incomplete, not blocking>
// SPEC:     <references a specific spec section for context>
```

---

## Go Conventions

**Errors:** Always wrap with context (`fmt.Errorf("doing X: %w", err)`). Never discard silently.
Sentinel errors for anything callers check with `errors.Is`. User-facing messages:
lowercase, no trailing punctuation, actionable.

**Packages:** Follow the structure in `SPEC.md` §7 exactly. No `utils`/`helpers` packages.
Don't create interfaces until there's a second implementation — except at the shell boundary
(`git`, `gh`, `code`) where a thin interface enables test fakes.

**Functions:** Accept interfaces, return structs. Options struct if >3 parameters.
Return `(T, error)` over panicking. Panic only for programmer errors.

**Testing:** Table-driven. `TestFunctionName_Scenario`. Use `testify/assert` + `testify/require`.
Test behavior, not implementation. Tests live next to the code they test.

**Concurrency:** `errgroup` for parallel ops, bounded by `[parallel].batch_size`.
No shared mutable state without synchronization.

---

## TUI Conventions

Each flow gets its own file in `internal/tui/`. Standard Bubble Tea model/update/view.
Use `bubbles` components over custom implementations. Styles defined once in `styles.go`.

Enter confirms, Escape cancels, q quits. Show keybindings at the bottom. Fast by default —
no unnecessary loading screens.

---

## Don't

- Add features not in the spec. Flag gaps instead.
- Create interfaces preemptively (except shell boundaries for testing).
- Create abstraction layers "for the future."
- Add configuration for things the spec says are hardcoded (root folder, ergo key structure).
- Optimize before it's correct. State cache is the only v1 perf optimization.
- Wrap errors without adding context.