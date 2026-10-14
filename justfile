# Go packages: explicit, so stray .go files elsewhere (e.g. tmp/) are never picked up.
go_pkgs := "./cmd/... ./internal/... ./ui"

[private]
default: help

# List the available recipes
help:
    @just --list --unsorted

# Install the UI's dependencies (once, and after pulling)
[group('dev')]
setup:
    cd ui && bun install

# Run Foreman on a workspace, rebuilding the UI on every change (reload the page to see it)
[group('dev')]
dev workspace=".dev/workspace":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{workspace}}"
    # Build once in the foreground, so a broken UI toolchain stops here with its own error.
    (cd ui && bun run build)
    (cd ui && bun run watch) &
    trap 'kill $! 2>/dev/null' EXIT
    go run ./cmd/foreman "{{workspace}}" --yes --ui-dir ui/dist/app

# Build the single `foreman` binary, with the web UI embedded, into bin/
[group('build')]
build:
    cd ui && bun run build
    go build -o bin/foreman ./cmd/foreman

# Run every test
[group('quality')]
test:
    go test {{go_pkgs}}

# Format check, vet, staticcheck and the web linter
[group('quality')]
lint:
    @test -z "$(gofmt -l cmd internal ui/embed.go)" || (gofmt -l cmd internal ui/embed.go && echo "gofmt: files above need formatting" && exit 1)
    go vet {{go_pkgs}}
    go tool staticcheck {{go_pkgs}}
    cd ui && bun run lint

# Run any wiki command against the knowledge base, e.g. `just wiki status`
[group('wiki')]
wiki *args:
    wiki --root wiki {{args}}

# Health check of the knowledge base (errors fail; broken links are warnings)
[group('wiki')]
check:
    wiki --root wiki check
