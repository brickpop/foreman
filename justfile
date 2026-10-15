# Go packages: explicit, so stray .go files elsewhere (e.g. tmp/) are never picked up.
go_pkgs := "./cmd/... ./internal/... ./ui ./images"

[private]
default: help

# List the available recipes
help:
    @just --list --unsorted

# Install the UI's dependencies (once, and after pulling)
[group('dev')]
setup:
    cd ui && bun install

# Run Foreman on a workspace with one demo session (the fake harness, or the command you pass), rebuilding the UI on every change (reload the page to see it)
[group('dev')]
dev workspace=".dev/workspace" session="":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{workspace}}/.foreman"
    # Development runs agents as host processes: no Podman machine to start.
    [ -f "{{workspace}}/.foreman/settings.toml" ] || echo 'engine = "direct"' > "{{workspace}}/.foreman/settings.toml"
    session="{{session}}"
    if [ -z "$session" ]; then go build -o .dev/fakeharness ./cmd/fakeharness; session=.dev/fakeharness; fi
    # Build once in the foreground, so a broken UI toolchain stops here with its own error.
    (cd ui && bun run build)
    (cd ui && bun run watch) &
    trap 'kill $! 2>/dev/null' EXIT
    go run ./cmd/foreman "{{workspace}}" --yes --ui-dir ui/dist/app --demo-session "$session"

# Regenerate the UI's TypeScript types from the Go structs (tygo.yaml); commit the result
[group('build')]
types:
    go tool tygo generate

# Build the single `foreman` binary, with the web UI embedded, into bin/
[group('build')]
build:
    cd ui && bun run build
    go build -o bin/foreman ./cmd/foreman

# Run every test
[group('quality')]
test:
    go test {{go_pkgs}}
    cd ui && bun run test

# Run the Podman engine's integration tests against the real Podman (starts the service or machine if needed; pulls alpine)
[group('quality')]
test-podman:
    go test -tags podman -count=1 ./internal/runner/podman/

# Run the Go tests with the race detector (needs a C compiler, e.g. on the Mac)
[group('quality')]
race:
    CGO_ENABLED=1 go test -race {{go_pkgs}}

# Format check, vet, staticcheck, generated types up to date, and the web linter
[group('quality')]
lint:
    @test -z "$(gofmt -l cmd internal ui/embed.go images)" || (gofmt -l cmd internal ui/embed.go images && echo "gofmt: files above need formatting" && exit 1)
    go vet {{go_pkgs}}
    go tool staticcheck {{go_pkgs}}
    @before="$(cat ui/src/api/types/*.ts | cksum)"; go tool tygo generate; [ "$before" = "$(cat ui/src/api/types/*.ts | cksum)" ] || (echo "ui/src/api/types was stale: regenerated it, review the change" && exit 1)
    cd ui && bun run lint

# Health check of the knowledge base (errors fail; broken links are warnings)
[group('wiki')]
check:
    wiki --root wiki check
