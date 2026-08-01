# Release Notes

One file per released tag, named exactly after it: `v<X.Y.Z>.md` (e.g.
[v0.2.0.md](v0.2.0.md)). The file is both the permanent in-repo record and the
body of the GitHub release (attached after tagging via
`gh release edit v<X.Y.Z> --notes-file ergo-docs/release-notes/v<X.Y.Z>.md`).

The full release flow lives in
[08-build-test-release.md → Cutting a release](../08-build-test-release.md#cutting-a-release).

## When to write

**Before tagging** — in the same PR as (or before) the last change going into
the release, so the notes ship inside the tagged commit. If notes for the
upcoming version don't exist yet when you're merging a user-visible change,
create the file and add your entry; the version heading can be corrected right
before tagging if the bump size changes.

## Structure

```markdown
# ergo v<X.Y.Z>

Released: <YYYY-MM-DD>

<One- or two-sentence framing: what this release is about, and the upgrade
command(s) if worth calling out.>

## Breaking            ← only when applicable; always first
## New: <feature name>  ← one section per feature
## Improved: <behavior> ← behavior changes to existing commands
## Fixed                ← bug fixes, bullet list
## Internal             ← refactors, test/CI work, docs; terse bullets
```

Omit any section with nothing in it. See [v0.2.0.md](v0.2.0.md) as the
reference example.

## Writing guidelines

- **Write for users, not contributors.** Lead with the problem the change
  solves ("you were prompted for a username on every sync"), then the feature.
  Commit-level detail belongs in git history, not here.
- **Show, don't describe.** Config settings get a TOML snippet; CLI behavior
  changes get a before/after or sample output block.
- **State the default and the upgrade impact.** Every new setting documents
  its default and what happens to users who do nothing ("upgrading changes
  nothing until you opt in").
- **Call out migration steps** when existing state isn't auto-migrated (e.g.
  existing clones keeping their old `origin` remote), with the exact command
  to run.
- **Date and tag must match reality**: `Released:` is the day the tag is
  pushed; the `# ergo v<X.Y.Z>` heading matches the file name and the git tag.
