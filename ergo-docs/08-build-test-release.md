# Build, Test, Release

## Build

For local development the project builds a single binary for **macOS arm64**.
Releases are cross-compiled to the full matrix ({darwin, linux, windows} ×
{amd64, arm64}) by [GoReleaser](https://goreleaser.com) — see
[Cutting a release](#cutting-a-release).

```bash
just build       # GOOS=darwin GOARCH=arm64 go build -ldflags ... -o bin/ergo .
just install     # go install -ldflags ... .  (into ~/go/bin)
```

Version is embedded at build time:

```
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o bin/ergo .
```

The `version` variable in `main.go` defaults to `"dev"` when the linker flag
isn't set. `main.main` calls `cmd.Execute(version)` which sets both
`cmd.version` and `rootCmd.Version`.

## justfile recipes

[`justfile`](../../ergo/justfile) is the canonical task runner. Run `just --list`
to see all targets. Highlights:

| Recipe                  | Action                                       |
| ----------------------- | -------------------------------------------- |
| `just build`            | Build the binary for darwin/arm64            |
| `just install`          | `go install` with version ldflag             |
| `just clean`            | Remove `bin/` and clear Go caches            |
| `just release <tag>`    | `git tag <tag>` + push (goreleaser runs in CI) |
| `just release-check`    | validate `.goreleaser.yaml`                  |
| `just release-snapshot` | local matrix + formula + winget manifests into `dist/` |
| `just test`             | `go test ./...`                              |
| `just test-v`           | verbose                                      |
| `just test-race`        | with race detector                           |
| `just test-pkg <p>`     | scoped: `go test -v ./<p>/...`               |
| `just test-run <regex>` | filter by name                               |
| `just test-cover`       | coverage HTML report                         |
| `just vet`              | `go vet ./...`                               |
| `just fmt`              | `gofmt -w . && goimports -w .`               |
| `just fmt-check`        | CI-friendly check                            |
| `just tidy`             | `go mod tidy && go mod verify`               |
| `just check`            | full pre-commit: fmt + vet + test-race       |
| `just run -- ...`       | `go run -ldflags ... . <args>`               |

The `version` variable derives from `git describe --tags --always --dirty`.

## Continuous Integration

Two GitHub Actions workflows in [`.github/workflows/`](../../ergo/.github/workflows/):

### `ci.yml` — on push/PR to `main`

1. Checkout
2. `actions/setup-go@v5` with `go-version-file: go.mod`
3. `go mod tidy` → `git diff --exit-code` (fails if `go.mod` / `go.sum` would change)
4. `go vet ./...`
5. `gofmt -l .` must produce empty output
6. `go test -race -coverprofile=coverage.out ./...`
7. Build the **full release matrix** (`{darwin,linux,windows}` ×
   `{amd64,arm64}`) as a cross-compile smoke test.

The `integration` job builds the Docker image and runs the end-to-end suite.

### `release.yml` — on push of a `v*` tag

1. Checkout with `fetch-depth: 0` (needed for version/release notes)
2. Setup Go from `go.mod`
3. `go test -race ./...`
4. `goreleaser/goreleaser-action@v6` runs `goreleaser release --clean`, which:
   - cross-compiles the full build matrix;
   - emits per-platform raw binaries named `ergo-<goos>-<goarch>` (`.exe` on
     Windows) **and** archives (`.tar.gz` on Unix, `.zip` on Windows);
   - writes a single `checksums.txt`;
   - creates the GitHub release with all assets;
   - commits the generated formula to
     [`juan7732/homebrew-tap`](https://github.com/juan7732/homebrew-tap); and
   - generates the winget manifests for `juan7732.ergo` from the Windows zips,
     pushes them to a branch on the `juan7732/winget-pkgs` fork, and opens a
     pull request into `microsoft/winget-pkgs` (see
     [winget channel](#winget-channel)).

The raw asset name `ergo-<goos>-<goarch>` matches the name `ergo update` derives
from `runtime.GOOS`/`runtime.GOARCH`, and the manifest name `checksums.txt`
matches `github.ChecksumName` — `ergo update` downloads the matching asset and
verifies its SHA-256 against `checksums.txt` before swapping. Keep
[`.goreleaser.yaml`](../../ergo/.goreleaser.yaml) and
[`cmd/update.go`](../../ergo/cmd/update.go) in sync.

**Required secrets** (repository secrets on `juan7732/ergo`; the default
`GITHUB_TOKEN` is scoped to the `ergo` repo only and cannot write to either
external repository):

| Secret               | What it must be able to do                                                                                          |
| -------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `HOMEBREW_TAP_TOKEN` | PAT with write access to `juan7732/homebrew-tap`, so the `brews` block can commit the formula.                        |
| `WINGET_PKGS_TOKEN`  | **Classic** PAT with the `public_repo` scope. Pushes the manifest branch to the `juan7732/winget-pkgs` fork and opens the pull request into `microsoft/winget-pkgs`. A fine-grained token cannot open pull requests on repositories outside its owner, so it will not work here. |

### winget channel

Windows users get ergo through the community repository
[`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs) as the
package `juan7732.ergo`. The `winget:` block in
[`.goreleaser.yaml`](../../ergo/.goreleaser.yaml) is purely additive: it reads
the existing `ergo_<version>_windows_<arch>.zip` archives and emits a three-file
manifest set (version, installer, default locale, schema 1.12.0) declaring a
**portable** package with the command alias `ergo`:

```
manifests/j/juan7732/ergo/<version>/
  juan7732.ergo.yaml                 # version
  juan7732.ergo.installer.yaml       # InstallerType: zip, NestedInstallerType: portable,
                                     # PortableCommandAlias: ergo, x64 + arm64 zips + SHA-256
  juan7732.ergo.locale.en-US.yaml    # publisher, description, license, tags
```

On every tag goreleaser pushes those files to a branch `ergo-<version>` on the
fork and opens a pull request against `microsoft/winget-pkgs`. That PR is not
merged by ergo's pipeline: winget-pkgs runs its own validation and a human
moderator approves it, after which it auto-merges. Pre-release tags publish a
GitHub release but skip the winget PR (`skip_upload: auto`).

One-time prerequisites, outside the repository:

1. Fork `microsoft/winget-pkgs` as `juan7732/winget-pkgs`
   (`gh repo fork microsoft/winget-pkgs --clone=false`). goreleaser syncs the
   fork with upstream before pushing, but it cannot create it.
2. Create the `WINGET_PKGS_TOKEN` secret (see the table above).

**First submission.** A brand-new package gets more scrutiny than a version
bump and can take days to merge. Until the first PR is merged,
`winget install juan7732.ergo` does not work and the README must say so;
[issue #6](https://github.com/juan7732/ergo/issues/6) tracks flipping the
docs once it is live. If a moderator asks for manifest changes, edit the
files on the fork branch by hand and push; goreleaser's next tag regenerates
from `.goreleaser.yaml`, so fold any permanent change back into that block.

**Known `just release-check` caveat.** goreleaser 2.16+ hard-deprecates the
`brews` block this project deliberately keeps (see the `// DECISION` above it),
and `goreleaser check` now exits 2 whenever a deprecated key is present, even
though it reports the configuration as valid. Until the formula-vs-cask
question is revisited, treat "configuration is valid, but uses deprecated
properties" as a pass and anything else as a failure. `goreleaser release`
itself only warns.

## Tests

Unit tests live next to the code they test:

| Package              | Test file                                                                     |
| -------------------- | ----------------------------------------------------------------------------- |
| `internal/config`    | `config_test.go` — `DeriveRepoName`, validation, defaults                     |
| `internal/git`       | `git_test.go` — runner-based tests with fakes                                 |
| `internal/vscode`    | `generate_test.go`, `diff_test.go` — golden-file generation, write-if-changed |
| `internal/workspace` | `detect_test.go`, `resolve_test.go`, `filter_test.go`, `runner_test.go`       |

Patterns:

- Table-driven (`tests := []struct{...}{...}`).
- `TestFunctionName_Scenario` naming.
- `testify/assert` + `testify/require` per project conventions.
- Temp dirs via `t.TempDir()` — automatic cleanup.
- `git` calls use a fake `Runner` so tests never spawn real git.

## Integration suite

Lives under [`test/integration/`](../../ergo/test/integration/) and is
**Docker-based** so it runs hermetically against real `git`, a stub `gh`, and
a stub `code`. Compatible with Docker Desktop, OrbStack, and any compose-aware
runtime.

### `Dockerfile`

Two-stage:

1. **Build stage** (`golang:1.26-bookworm`): `go build -ldflags "-X main.version=integration" -o /out/ergo .`
2. **Runtime stage** (`debian:stable-slim`): installs `git`, `bash`, `ca-certificates`,
   configures global git identity, copies the Go toolchain over from the build
   image (so the in-container test runner can compile the integration package),
   copies the `ergo` binary into `/usr/local/bin/`, and sets the default
   command to `go test -tags=integration -count=1 ./test/integration/...`.

### `harness/` package

Build-tagged `integration` (so it doesn't build in normal `go test ./...`).

| File             | Purpose                                                                                                                                                                             |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `harness.go`     | `Harness` struct: per-test `HOME`, `PATH` prefix dir, `Run/RunIn/RunWith` to invoke the in-container binary, helpers for reading/writing workspace TOMLs and asserting on `Result`. |
| `gitfixtures.go` | Local fixture-repo helpers so tests can `git clone` from a `git://` URL or local path without touching the network.                                                                 |
| `stubs.go`       | Fake `gh` and `code` binaries the harness writes into `PathDir` so `update` tests can serve canned release tags, assets, and a matching (or deliberately corrupt) `checksums.txt`, and `open`/`edit` tests can record `code` invocations. |

`Result` carries `Stdout`, `Stderr`, `Combined`, `ExitCode`, `Err`, plus
`AssertOK(t)` / `AssertFail(t)` helpers.

The harness deliberately sets `NO_COLOR=1` and `TERM=dumb` to keep string
assertions stable across environments.

### Running

The dockerized suite runs via `just`:

```bash
just integration         # build the image + run the end-to-end suite
just integration-race    # same, with the race detector
just integration-shell   # interactive shell in the container for debugging
```

These recipes drive `docker build`/`docker run` against
[`test/integration/Dockerfile`](../../ergo/test/integration/Dockerfile)
directly — there is no `Makefile` or `docker-compose.yml`.

### TUI scope explicitly excluded

End-to-end TUI flows (`init`, `add`, `remove`, `show` no-arg) are tested via
`teatest` unit tests in their own package, not by the docker suite. Shorthand
non-interactive paths (`ergo add repo <url>`, `ergo show <group>`) are covered
by integration tests. See the implementation plan §10 for the rationale.

## Pre-commit ritual

Per `.github/copilot-instructions.md`:

```bash
just check       # fmt + vet + test-race
```

…must pass before committing. The custom `commit-push` skill at
[`.github/skills/commit-push/SKILL.md`](../../ergo/.github/skills/commit-push/SKILL.md)
automates the workflow: tidy, fmt, vet, test-race, review staged diff, write
a Conventional Commit message, push to current branch (refuses to push to
`main` without confirmation).

## Cutting a release

The release is **tag-driven**: pushing a `v*` tag triggers
[`release.yml`](../../ergo/.github/workflows/release.yml), which re-runs the
race-enabled test suite and then GoReleaser cross-compiles the matrix,
publishes the GitHub release (raw `ergo-<os>-<arch>` binaries, `.tar.gz`
archives, and `checksums.txt`), commits the updated formula to
`juan7732/homebrew-tap`, and opens a winget manifest pull request against
`microsoft/winget-pkgs`. Homebrew users then `brew upgrade ergo`;
standalone-binary users run `ergo update`; winget users `winget upgrade ergo`
once the PR has been moderated and merged.

The version number comes **only** from the tag (embedded via ldflags) — there
is no version constant to bump in code.

The `cut-release` skill at
[`.github/skills/cut-release/SKILL.md`](../.github/skills/cut-release/SKILL.md)
automates this whole section: preflight, version selection, notes
validation, tag + push, notes attachment, and verification. This document
remains canonical; the skill defers to it.

### 1. Pick the version

Semantic versioning against the previous tag (`git tag --sort=-v:refname`):

- **minor** (`v0.2.0` → `v0.3.0`): new features, new config settings, new
  commands — anything additive.
- **patch** (`v0.2.0` → `v0.2.1`): bug fixes and doc-only corrections with no
  behavior additions.
- Backward-incompatible changes (config keys removed/renamed, command
  semantics changed) warrant a minor bump while pre-1.0, and a prominent
  "Breaking" section in the release notes.

### 2. Write the release notes — before tagging

Create `ergo-docs/release-notes/v<X.Y.Z>.md` **in the same PR as (or before)
the last feature going into the release**, so the notes ship inside the tagged
commit. Follow the conventions in
[release-notes/README.md](release-notes/README.md).

### 3. Land everything on `main` via PR

Work happens on feature branches; `ci.yml` must be green (Test & Lint +
dockerized Integration) before merge. Pre-flight locally:

```bash
just check         # fmt + vet + test-race
just integration   # dockerized end-to-end suite
```

Optionally dry-run the release pipeline itself (requires `goreleaser` on PATH):

```bash
just release-check       # validate .goreleaser.yaml (see the brews caveat under "winget channel")
just release-snapshot    # build matrix + formula + winget manifests into dist/ without publishing
```

After a snapshot, inspect `dist/winget/manifests/j/juan7732/ergo/<version>/`:
the installer manifest must list x64 and arm64 entries pointing at the
`ergo_<version>_windows_<arch>.zip` release URLs with `PortableCommandAlias:
ergo`.

### 4. Tag from a clean, up-to-date main

```bash
git checkout main && git pull    # HEAD must be the merge commit you mean to ship
just release v0.3.0              # git tag + push → triggers release.yml
```

Watch the run: `gh run watch $(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')`.

### 5. Attach the curated notes to the GitHub release

GoReleaser generates a bare commit-list changelog as the release body. Replace
it with the curated notes:

```bash
gh release edit v0.3.0 --notes-file ergo-docs/release-notes/v0.3.0.md
```

### 6. Verify

- **Assets** — `gh release view v0.3.0 --json assets` should list 13 assets:
  6 raw binaries, 6 archives, and `checksums.txt`.
- **Homebrew tap** — the formula in
  [`juan7732/homebrew-tap`](https://github.com/juan7732/homebrew-tap) shows the
  new `version` and fresh SHA-256s (an automated
  "Brew formula update for ergo version v0.3.0" commit).
- **Upgrade path** — spot-check `brew upgrade ergo && ergo --version`, or
  `ergo update` from a previous standalone binary.
- **winget**: the release job's summary links the pull request it opened
  against `microsoft/winget-pkgs` (title `New version: juan7732.ergo <version>`);
  or find it with
  `gh pr list --repo microsoft/winget-pkgs --author juan7732 --search juan7732.ergo`.
  Watch its labels: `Validation-Completed` means the manifests passed the
  winget-pkgs pipeline; `Needs-Author-Feedback` means a moderator wants a
  change (edit the `ergo-<version>` branch on the fork). It merges on its own
  once labelled `Moderator-Approved`. Do not count the PR as done at tag time;
  for the first submission expect days, not minutes. When it has merged,
  `winget install juan7732.ergo` (or `winget upgrade ergo`) on a Windows
  machine is the end-to-end check.

### If the release fails

- **Workflow failed before publishing** (tests, build): fix on `main` via PR,
  then delete and re-push the tag —
  `git tag -d v0.3.0 && git push origin :refs/tags/v0.3.0`, retag with
  `just release v0.3.0`.
- **Release already published**: never reuse or move the tag — Homebrew and
  `ergo update` have both seen the checksums. Cut a patch release instead.
- **Only the winget step failed** (fork missing, token rejected, PR could
  not be opened): the GitHub release and the Homebrew formula are already
  correct; nothing needs re-tagging. Fix the prerequisite, then open the PR
  by hand from the manifests goreleaser wrote (re-run `just release-snapshot`
  at the tag, or copy them from the job log) onto an `ergo-<version>` branch
  of the fork.
