# ergo — development task runner
# Install just: brew install just
# Usage: just <recipe>    |    just --list

# Default recipe: show available commands
default:
    @just --list

# ─── Build ────────────────────────────────────────────────────────────────────

# Build the binary for macOS arm64
build:
    GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version={{version}}" -o bin/ergo .

# Build and install to ~/go/bin
install:
    go install -ldflags "-X main.version={{version}}" .

# Clean build artifacts
clean:
    rm -rf bin/
    go clean -cache -testcache

# Tag and push a release (goreleaser runs in CI on the pushed tag)
release tag:
    git tag {{tag}}
    git push origin {{tag}}
    @echo "Release {{tag}} triggered — check Actions tab for progress"

# Validate the goreleaser config (requires goreleaser on PATH).
release-check:
    goreleaser check

# Local dry-run: build the full matrix + Homebrew formula into dist/ without
# publishing. Useful before cutting a tag. Requires goreleaser on PATH.
release-snapshot:
    HOMEBREW_TAP_TOKEN=dummy goreleaser release --snapshot --clean --skip=publish

# ─── Test ─────────────────────────────────────────────────────────────────────

# Run all tests
test:
    go test ./...

# Run all tests with verbose output
test-v:
    go test -v ./...

# Run all tests with race detector
test-race:
    go test -race ./...

# Run tests for a specific package (e.g., just test-pkg internal/config)
test-pkg pkg:
    go test -v ./{{pkg}}/...

# Run tests matching a pattern (e.g., just test-run TestResolve)
test-run pattern:
    go test -v -run {{pattern}} ./...

# Run tests with coverage report
test-cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# ─── Lint & Format ───────────────────────────────────────────────────────────

# Run go vet
vet:
    go vet ./...

# Format all Go files
fmt:
    gofmt -w .
    goimports -w .

# Check formatting without modifying (CI-friendly)
fmt-check:
    @test -z "$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# ─── Dependencies ────────────────────────────────────────────────────────────

# Tidy and verify modules
tidy:
    go mod tidy
    go mod verify

# ─── Dev Workflow ─────────────────────────────────────────────────────────────

# Full check: fmt, vet, test with race detector
check: fmt vet test-race

# Quick iteration: build + run with args (e.g., just run -- status)
run *args:
    go run -ldflags "-X main.version={{version}}" . {{args}}

# ─── Integration (Docker) ────────────────────────────────────────────────────

# Image tag for the integration test image. Override with: just integration tag=foo
integration_tag := "ergo-integration:latest"

# Build the integration image and run the dockerized end-to-end suite.
# Requires Docker (Docker Desktop, OrbStack, etc.).
# Uses -v so individual test names are visible — the suite runs in parallel
# and finishes in <1s, which otherwise looks like nothing ran.
integration:
    docker build -f test/integration/Dockerfile -t {{integration_tag}} .
    docker run --rm \
        -v "$PWD":/src \
        {{integration_tag}} \
        go test -tags=integration -v -count=1 ./test/integration

# Same as `integration` but with the race detector enabled.
integration-race:
    docker build -f test/integration/Dockerfile -t {{integration_tag}} .
    docker run --rm \
        -v "$PWD":/src \
        {{integration_tag}} \
        go test -tags=integration -race -v -count=1 ./test/integration

# Open an interactive shell in the integration image for debugging.
integration-shell:
    docker build -f test/integration/Dockerfile -t {{integration_tag}} .
    docker run --rm -it \
        -v "$PWD":/src \
        --entrypoint /bin/bash \
        {{integration_tag}}

# ─── Version ──────────────────────────────────────────────────────────────────

# Git-derived version: tag if available, otherwise short SHA
version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
