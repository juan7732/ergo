---
name: cut-release
description: 'Cut an ergo release end to end: pick the version, validate the release notes, run preflight checks, tag and push (CI runs GoReleaser), attach the curated notes, and verify assets, the Homebrew tap, and the winget-pkgs pull request. Use when the user asks to cut, tag, or ship a release, or publish a new version of ergo.'
argument-hint: '[version, e.g. v0.5.0]'
---

# Cut a Release

Tag-driven release flow. Pushing a `v*` tag triggers `release.yml`, which
re-runs the race-enabled tests, then GoReleaser cross-compiles the matrix,
publishes the GitHub release, commits the formula to
`juan7732/homebrew-tap`, and opens a winget manifest pull request against
`microsoft/winget-pkgs` from the `juan7732/winget-pkgs` fork. The version
comes only from the tag; there is no version constant in code.

Canonical documentation: `ergo-docs/08-build-test-release.md` (section
"Cutting a release") and `ergo-docs/release-notes/README.md`. This skill
operationalizes them; if they ever disagree with this file, the docs win
and this skill needs updating.

Publishing a release is outward-facing and irreversible once assets are
downloaded. Confirm the version with the user before tagging.

## Phase 0 — Preflight

Run in parallel:

```bash
git status --short                      # must be empty
git rev-parse --abbrev-ref HEAD         # must be main
git fetch origin && git rev-parse HEAD origin/main   # must match
git tag --sort=-v:refname | head -5     # previous versions
gh auth status
```

Stop and report if: the tree is dirty, HEAD is not `main`, local main is
behind origin, or `gh` is not authenticated. Then confirm CI is green on
HEAD:

```bash
gh run list --branch main --limit 3
```

## Phase 1 — Pick and confirm the version

Semantic versioning against the latest tag:

- **minor** (`v0.4.0` → `v0.5.0`): anything additive (new commands, flags,
  config settings, features).
- **patch**: bug fixes and doc-only corrections, no behavior additions.
- Backward-incompatible changes: minor bump while pre-1.0, plus a
  prominent "Breaking" section first in the notes.

Cross-check the bump against what actually changed since the last tag
(`git log <last-tag>..HEAD --oneline`). If the user supplied a version
that disagrees with the change set (e.g. patch bump but new commands
landed), stop and surface the mismatch instead of proceeding.

## Phase 2 — Validate the release notes

The notes must exist BEFORE tagging, inside the commit being tagged:

1. `ergo-docs/release-notes/v<X.Y.Z>.md` exists.
2. Its `# ergo v<X.Y.Z>` heading matches the file name and the tag.
3. `Released:` is today's date. If it is wrong, fix it now; the change
   must land on `main` before tagging (notes ship inside the tagged
   commit).
4. Structure follows `release-notes/README.md`: Breaking first when
   applicable, then New / Improved / Fixed / Internal; no empty sections;
   written for users (problem first, TOML/CLI examples, defaults and
   upgrade impact stated).
5. Roadmap graduation: items shipping in this release are removed from
   `ergo-docs/11-roadmap.md` (the roadmap rule: shipped items live in
   release notes, not the roadmap). If one was missed, fix it in the same
   pre-tag commit.

## Phase 3 — Local verification

```bash
just check         # fmt + vet + test-race
just integration   # dockerized end-to-end suite (Docker/OrbStack required)
```

Optional but recommended when `.goreleaser.yaml` changed since the last
release (requires goreleaser on PATH):

```bash
just release-check      # validate .goreleaser.yaml
just release-snapshot   # build matrix + formula + winget manifests into dist/ without publishing
```

`release-check` currently exits 2 with "configuration is valid, but uses
deprecated properties" because of the deliberately kept `brews` block; that
exact message is a pass. Any other error is a failure.

Stop on any failure. Never tag around a red check.

## Phase 4 — Tag and watch

Confirm the exact version string with the user, then:

```bash
git checkout main && git pull   # HEAD must be the commit you mean to ship
just release v<X.Y.Z>           # git tag + push, triggers release.yml
gh run watch $(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```

## Phase 5 — Attach the curated notes

GoReleaser writes a bare commit-list changelog as the release body.
Replace it:

```bash
gh release edit v<X.Y.Z> --notes-file ergo-docs/release-notes/v<X.Y.Z>.md
```

## Phase 6 — Verify

- **Assets**: `gh release view v<X.Y.Z> --json assets --jq '.assets | length'`
  should report 13 (6 raw `ergo-<os>-<arch>` binaries, 6 archives,
  `checksums.txt`).
- **Homebrew tap**: `juan7732/homebrew-tap` has a fresh
  "Brew formula update for ergo version v<X.Y.Z>" commit with the new
  version and SHA-256s (`gh api repos/juan7732/homebrew-tap/commits --jq '.[0].commit.message'`).
- **Upgrade path**: spot-check `brew upgrade ergo && ergo --version`, or
  `ergo update` from a standalone binary.
- **winget PR**: the release job opened a pull request against
  `microsoft/winget-pkgs` titled `New version: juan7732.ergo <version>`
  (`gh pr list --repo microsoft/winget-pkgs --author juan7732 --search juan7732.ergo`).
  It is moderated and merged by winget-pkgs, not by us: report its URL and
  current labels, and tell the user to watch for `Needs-Author-Feedback`.
  If no PR exists, the winget step failed; see "If the release fails".
  The 13-asset count above is unchanged by winget (manifests live in
  winget-pkgs, not on the release).

Report a summary: version, release URL, asset count, tap commit, winget PR
URL and state, and any verification the user should do by hand.

## If the release fails

| Situation | Action |
|---|---|
| Workflow failed BEFORE publishing (tests, build) | Fix on `main` via PR, then `git tag -d v<X.Y.Z> && git push origin :refs/tags/v<X.Y.Z>`, retag with `just release v<X.Y.Z>`. |
| Release already published | Never reuse or move the tag; Homebrew and `ergo update` have seen the checksums. Cut a patch release instead. |
| Notes wrong after publishing | `gh release edit` the body freely; the in-repo notes file needs a normal PR. |
| Only the winget step failed (no PR opened) | Release and formula are fine; do not re-tag. Fix the prerequisite (fork `juan7732/winget-pkgs`, `WINGET_PKGS_TOKEN` secret), then open the PR by hand from the generated manifests per `08-build-test-release.md`. |

## Hard rules

- Never move, delete, or reuse a published tag.
- Never tag with a dirty tree, from a branch, or behind origin/main.
- Notes before tag, always; `Released:` date matches the day the tag is
  pushed.
- Never force-push anything as part of a release.
- Asset names (`ergo-<goos>-<goarch>`, `checksums.txt`) are a contract
  with `ergo update` and the formula; changing `.goreleaser.yaml`
  templates is release-engineering work, not part of cutting a release.
