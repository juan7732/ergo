# Build, Test, Release

## Build

The project builds as a single static-ish binary for **macOS arm64**.

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
| `just release <tag>`    | `git tag <tag>` + push (CI handles the rest) |
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
7. `GOOS=darwin GOARCH=arm64 go build` (smoke test the cross-build)

### `release.yml` — on push of a `v*` tag

1. Checkout with `fetch-depth: 0` (needed for `git describe`/release notes)
2. Setup Go from `go.mod`
3. `go test -race ./...`
4. `go build -ldflags "-X main.version=${{ github.ref_name }}" -o ergo-darwin-arm64 .`
5. `softprops/action-gh-release@v2` publishes `ergo-darwin-arm64` as a release
   asset with auto-generated release notes.

The release asset name `ergo-darwin-arm64` matches the `assetName` constant in
[`cmd/update.go`](../../ergo/cmd/update.go) — keep them in sync.

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
| `stubs.go`       | Fake `gh` and `code` binaries the harness writes into `PathDir` so `update` tests can serve canned release JSON and `open`/`edit` tests can record `code` invocations.              |

`Result` carries `Stdout`, `Stderr`, `Combined`, `ExitCode`, `Err`, plus
`AssertOK(t)` / `AssertFail(t)` helpers.

The harness deliberately sets `NO_COLOR=1` and `TERM=dumb` to keep string
assertions stable across environments.

### Running

The implementation plan defines:

```bash
make integration         # docker compose run --rm ergo-test
make integration-shell   # interactive shell in the container for debugging
```

(The `Makefile` and `docker-compose.yml` referenced in the plan are not yet
present in the tree — they belong to phase 10 and may still be in progress.
The Dockerfile and harness are in place.)

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

```bash
just release v0.2.0
# triggers .github/workflows/release.yml on the pushed tag
```

The workflow builds, tests, and uploads the binary. Users can then run
`ergo update` to fetch it.
