# Go packages: explicit, so stray .go files elsewhere (e.g. tmp/) are never picked up.
go_pkgs := "./cmd/... ./internal/... ./ui ./images ./agentbin"

[private]
default: help

# List the available recipes
help:
    @just --list --unsorted

# Install the UI's dependencies (once, and after pulling)
[group('dev')]
setup:
    cd ui && bun install

# Run Foreman with the UI rebuilt on every change (reload the page to see it). `just dev`: Claude Code on Podman, its home in .dev/real and its Foreman folder in .dev/real-folder. `just dev fake`: the fake harness (a stand-in that echoes, for staging cases without spending tokens) on the Direct engine, in .dev/fake and .dev/fake-folder. Each starts with team dev-team (Tic leads, Tac and Toe) and project dev-project (one code repo, some-app, in .dev/<harness>-repos). A home given second is used instead (it must be on the engine its harness needs)
[group('dev')]
dev harness="real" home="":
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{harness}}" in fake|real) ;; *) echo "just dev: the harness is fake or real, not '{{harness}}' (just dev [real|fake] [home])" >&2; exit 1;; esac
    # One home per harness, so each is always on the engine it needs. Its own
    # home, so it runs beside your own Foreman (one per home).
    home="{{home}}"; [ -n "$home" ] || home=".dev/{{harness}}"
    mkdir -p "$home"
    home="$(cd "$home" && pwd)"
    # Its own Foreman folder (HQ, projects, worktrees) beside the home, never
    # your ~/foreman.
    folder="$(dirname "$home")/$(basename "$home")-folder"
    # A new home: the fake harness runs as host processes (no Podman machine
    # to start); Claude Code runs in Podman.
    engine=direct; [ "{{harness}}" = real ] && engine=podman
    settings="$home/settings.toml"
    [ -f "$settings" ] || printf '%s\n' "engine = \"$engine\"" "foreman_folder = \"$folder\"" > "$settings"
    # Foreman makes its HQ around what is there at its first start.
    teams="$folder/hq/teams"
    if [ ! -d "$teams" ]; then
        mkdir -p "$teams"
        printf '%s\n' 'team = "dev-team"' 'name = "DevTeam"' 'lead = "tic"' '' \
            '[agents.tic]' 'name = "Tic"' 'role = "team_lead"' '' \
            '[agents.tac]' 'name = "Tac"' 'role = "engineer"' '' \
            '[agents.toe]' 'name = "Toe"' 'role = "reviewer"' > "$teams/dev-team.toml"
    fi
    # The code repo lives outside the home, as a CEO's repos do.
    app="$(dirname "$home")/$(basename "$home")-repos/some-app"
    if [ ! -d "$app" ]; then
        mkdir -p "$app"
        echo "# Some app repo" > "$app/README.md"
        git -C "$app" init --quiet --initial-branch=main
        git -C "$app" add README.md
        git -C "$app" -c user.name=dev -c user.email=dev@localhost -c commit.gpgsign=false commit --quiet -m "First commit"
    fi
    # The fake harness is a host program, which a container can't run; Claude Code
    # gets its login through a mount, which the Direct engine ignores.
    direct=no; grep -Eq '^engine *= *"direct"' "$settings" && direct=yes
    if [ "{{harness}}" = fake ] && [ $direct = no ]; then
        echo "just dev fake: $home runs on Podman, where the fake harness can't run. Use \`just dev\` for Claude Code, or set engine = \"direct\" in $settings." >&2
        exit 1
    fi
    if [ "{{harness}}" = real ] && [ $direct = yes ]; then
        echo "just dev: $home runs on the Direct engine, where Claude Code can't get its login. Set engine = \"podman\" in $settings (or in Settings, then restart), or use \`just dev fake\` for the fake harness." >&2
        exit 1
    fi
    fake=""
    if [ "{{harness}}" = "fake" ]; then
        go build -o .dev/fakeharness ./cmd/fakeharness
        fake="--fake-harness .dev/fakeharness"
    fi
    just agent
    # Build once in the foreground, so a broken UI toolchain stops here with its own error.
    (cd ui && bun run build)
    (cd ui && bun run watch) &
    watch=$!
    # Claude Code's logins and the image builds are yours, linked in.
    if [ "{{harness}}" = real ]; then
        for d in profiles images; do
            mkdir -p "$HOME/.foreman/$d"
            ln -sfn "$HOME/.foreman/$d" "$home/$d"
        done
    fi
    # Projects are records, made through the API: once the server answers,
    # dev-project is created with some-app, unless it exists.
    (
        for _ in $(seq 300); do
            sleep 0.2
            base="$(sed -nE 's#^(https?://[^/]+).*#\1#p' "$home/server.lock" 2>/dev/null)" || true
            token="$(cat "$home/access-token" 2>/dev/null)" || true
            [ -n "$base" ] && [ -n "$token" ] || continue
            api() { curl -sf -H "Authorization: Bearer $token" -H "Content-Type: application/json" "$@"; }
            api -o /dev/null "$base/api/home" || continue
            api -o /dev/null "$base/api/projects/dev-project" && exit 0
            api -o /dev/null -X POST "$base/api/projects" -d '{"id":"dev-project"}' &&
                api -o /dev/null -X POST "$base/api/projects/dev-project/repos" -d "{\"name\":\"Some app\",\"path\":\"$app\"}" &&
                exit 0
            echo "just dev: couldn't create dev-project through the API" >&2
            exit 1
        done
    ) &
    seed=$!
    trap 'kill $watch $seed 2>/dev/null' EXIT
    go run ./cmd/foreman --home "$home" --ui-dir ui/dist/app $fake

# Regenerate the UI's TypeScript types from the Go structs (tygo.yaml); commit the result
[group('build')]
types:
    go tool tygo generate

# Build the single `foreman` binary, with the web UI and foreman-agent embedded, into bin/ (the release build: it refuses to compile without them)
[group('build')]
build: agent
    cd ui && bun run build
    go test -tags release ./agentbin ./ui
    go build -tags release -o bin/foreman ./cmd/foreman

# Build foreman-agent for the agent containers (Linux, both CPU types), to embed in foreman
[group('build')]
agent:
    for arch in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath -ldflags "-s -w" -o agentbin/dist/foreman-agent-$arch ./cmd/foreman-agent || exit 1; done

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
    @test -z "$(gofmt -l cmd internal ui/*.go images agentbin)" || (gofmt -l cmd internal ui/*.go images agentbin && echo "gofmt: files above need formatting" && exit 1)
    go vet {{go_pkgs}}
    go tool staticcheck {{go_pkgs}}
    @before="$(cat ui/src/api/types/*.ts | cksum)"; go tool tygo generate; [ "$before" = "$(cat ui/src/api/types/*.ts | cksum)" ] || (echo "ui/src/api/types was stale: regenerated it, review the change" && exit 1)
    cd ui && bun run lint

# Health check of the knowledge base (errors fail; broken links are warnings)
[group('wiki')]
check:
    wiki --root wiki check
