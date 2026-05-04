# syntax=docker/dockerfile:1.7

# ─── Stage 1: build the ergo binary ───────────────────────────────────────────
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Cache deps independently of source.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build for the container's native arch (linux). The hardcoded
# ergo-darwin-arm64 release-asset name only matters for the update test, which
# stubs gh and serves a placeholder asset under that exact name.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -ldflags "-X main.version=integration" -o /out/ergo .

# ─── Stage 2: minimal runtime + test environment ──────────────────────────────
FROM debian:stable-slim AS runtime

# Real git is required (ergo shells out to it). bash is required for the
# gh/code stubs the tests install on PATH per scenario. ca-certificates is a
# safety net for any tools that need TLS roots.
RUN apt-get update \
    && apt-get install -y --no-install-recommends git bash ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Configure git so it can create commits in fixture repos.
RUN git config --global user.email "ergo-test@example.com" \
    && git config --global user.name "ergo-test" \
    && git config --global init.defaultBranch main

# We need the Go toolchain in the runtime image so we can run the integration
# test suite (which is itself a Go test package). Copy it from the build stage
# rather than re-installing.
COPY --from=golang:1.26-bookworm /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin
ENV GOPATH=/go
ENV GOCACHE=/root/.cache/go-build

COPY --from=build /out/ergo /usr/local/bin/ergo
RUN chmod 0755 /usr/local/bin/ergo

# Source tree is bind-mounted at runtime so `go test` can compile the
# integration package against the workspace it lives in.
WORKDIR /src

CMD ["go", "test", "-tags=integration", "-v", "-count=1", "./test/integration"]
