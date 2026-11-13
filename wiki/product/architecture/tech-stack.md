---
type: spec
status: partial
tags: [architecture, runtime, ui]
---

# Tech stack

The languages, libraries, repo layout, build and test setup. This entry is the single source of truth for "what do we use": other entries link here instead of naming libraries. Each choice says **why**, so that it can be challenged later. Every choice here is decided; the [Prove epic](../../backlog/epics/prove.md) confirms the runtime ones.

Guiding constraints, from the [vision](../vision.md): one local binary, local-first and offline-capable (no telemetry, no CDN calls), 20+ concurrent sessions on a laptop, macOS and Linux first, as few dependencies as possible, and every moving part behind a small swappable interface.

## Decided

| Decision | Why |
|---|---|
| **Go** for the server | The `wiki` engine ([Wiki engine](./wiki-engine.md)) is a Go module with public packages, so Foreman reuses its rules instead of reimplementing them. PTY handling, process supervision and a single static binary are Go's home ground, and the PTY proofs of concept were already in Go. |
| **One binary** serving the API and the built SPA | PRD requirement. The SPA is embedded with `//go:embed`. |
| **Every agent instance runs in a rootless Podman container** | Real isolation from day one: an agent sees only what is mounted ([Session runner](./session-runner.md), [Security](./security.md)). |
| **A swappable `Runner` interface** for where sessions run | Podman in production; the Direct engine (a pseudo-terminal on the host, no isolation) for hermetic tests and development. tmux or anything else can be added later without touching the rest ([Session runner](./session-runner.md)). |
| **Foreman builds the agent images as a four-layer chain** | base → harness → global → project, a layer without a Containerfile is just a tag of its parent; Refresh and Prune live in Settings → System → Maintenance ([Agent images](#agent-images)). |
| **Tailwind 4 + unstyled Radix primitives** for the UI | The prototype's tokens become CSS variables, Tailwind utilities sit over them, and Radix brings focus traps, Escape and keyboard navigation. |
| **JSON API typed from Go structs (`tygo`); PTY as raw binary frames** | See [Wire formats](#wire-formats). Protobuf adds nothing on the PTY path and solves a versioning problem we don't have. |
| **A per-install access token on every request**, delivered via the URL Foreman prints and opens at start | Listening on 127.0.0.1 doesn't stop websites in the browser or the agents' containers from reaching the API ([Security](./security.md#access-token)). |

## Server (Go)

| Concern | Choice | Why |
|---|---|---|
| Go version | The current stable release (1.27 at the time of writing) | Tracks stable; nothing needs a pinned older version. |
| HTTP routing | stdlib `net/http` (`ServeMux` method and path patterns) | Enough for ~30 endpoints; no framework. |
| Server-sent events | stdlib (`http.Flusher`) | A dozen lines; no library needed. |
| WebSocket (PTY streams) | `github.com/coder/websocket` | Small, context-aware, maintained; the stdlib has no WebSocket. |
| Pseudo-terminal (Direct engine) | `github.com/creack/pty` | The standard Go PTY package; used by the POCs. |
| Podman | The libpod REST API over the Podman socket, with plain `net/http` | Avoids the very heavy `containers/podman/v5/pkg/bindings` module. See [Session runner](./session-runner.md#podman-engine-production). |
| Team tools (agent tools) | MCP via `github.com/modelcontextprotocol/go-sdk` (official SDK): `foreman-agent` serves it to the harness over **stdio** (the form every harness supports) and relays to Foreman's agent port | Harnesses speak MCP natively; one shipped binary also carries the hook events ([Team tools → Transport](./team-tools.md#transport)) |
| Runtime state | SQLite via `modernc.org/sqlite` (pure Go) | No CGO, so the binary cross-compiles and stays static. WAL mode. |
| Queries and migrations | Hand-written `database/sql` in one `store` package; numbered `.sql` migrations embedded and applied by `PRAGMA user_version` | About 8 tables ([Persistence](./persistence.md)). An ORM or a code generator would cost more than it saves. |
| TOML (`project.toml`, team files, `settings.toml`) | `github.com/BurntSushi/toml` | Every Foreman config file is TOML, like `wiki.toml`. A save re-encodes the whole file (comments aren't kept); the [global repo](../models/home.md#the-global-repo) keeps the history. |
| TOML (`wiki.toml`) | Through the wiki engine (`bundle.DecodeTool`) | Never parsed twice. |
| Wiki | `github.com/agentic-wiki/wiki` (`bundle`, `index`, `parse`) | See [Wiki engine](./wiki-engine.md). |
| Server-side terminal state | `github.com/charmbracelet/x/vt`, pinned at v0.1.0, behind the `Screen` interface | A maintained, pure-Go VT emulator (screen, scrollback, alternate screen, modes, styled render) from the Charm team. Its first tagged release is v0.1.0 (2026-10-12); the post-1.0 alternatives are a parser only (`go-vte`) or unmaintained since 2020 without scrollback (`vt10x`). Round-trip tests guard upgrades ([Terminal state](./terminal-state.md)) |
| File watching | `github.com/fsnotify/fsnotify` | Config, team and wiki changes reach the UI within ~1s ([Persistence](./persistence.md)). |
| Git | The `git` CLI (on the host for Foreman's own operations, in the container for agents) | Same behaviour as the CEO's git; `go-git` lacks parts of worktrees and hooks. |
| Forges | Plain `net/http` clients: GitHub REST, and one Gitea client that also serves Forgejo (same API) | Three small adapters behind one interface ([Forge adapters](./forge-adapters.md)). |
| Logging | stdlib `log/slog` | Structured logs, no dependency. |
| CLI flags | stdlib `flag` | `foreman` has a handful of flags. |

Not used, on purpose: web frameworks, ORMs, dependency-injection containers, a message broker. In-process channels connect the components.

## UI

| Concern | Choice | Why |
|---|---|---|
| Language / framework | TypeScript, React 19 | Mature ecosystem for the two hard widgets (terminal, kanban). |
| Build / dev server | Vite | Fast dev server with a proxy to the Go server; static build to embed. |
| Package manager, scripts and runtime for the UI toolchain | Bun, and only Bun (never npm, yarn, pnpm or Node). `ui/bunfig.toml` sets `run.bun = true`, so Vite, TypeScript and Biome run on the Bun runtime even though their binaries declare `node` | The CEO's rule. Only for building the UI; nothing runs on Bun in production (the UI is embedded in the Go binary). |
| Server state | TanStack Query | Caching, refetching and optimistic updates (the board drag). SSE events invalidate or patch the cached queries, so there is one data path. |
| Routing | TanStack Router | Typed path and search params (project / section, with its agent, review, wiki entry or repo and branch; team / tab). |
| Styling | Tailwind CSS 4 with the [design tokens](../design/design-system.md) declared as CSS variables | The prototype is a token system with inline styles, which maps 1:1 onto Tailwind utilities over the same variables. Both themes switch by `data-theme` on `<html>`. |
| Accessible primitives | Radix UI primitives (dialog, popover, tabs, toggle), styled with our tokens | They bring what the prototype lacks: focus traps, Escape to close, real disabled states, keyboard navigation. Their default look is not used. |
| Terminal | `@xterm/xterm` with the fit, web-links and serialize addons | Real ANSI terminal, resize, clickable links; "serialize" backs the Copy button. |
| Wiki rendering | `react-markdown` with `remark-gfm` and a callout plugin, raw HTML off, as [wikiview](https://github.com/agentic-wiki/wikiview) does (the CEO, 2026-10-17: reuse its proven building blocks) | The server sends an entry's Markdown as on disk plus the engine's answers as lookup tables (each link resolved, heading ids, checkbox lines), so the browser renders without re-implementing a wiki rule ([Wiki engine](./wiki-engine.md#rendering)). Wiki content is written by agents, so it is untrusted: no raw HTML, script URLs dropped, and only the wiki's own images load ([Security](./security.md)). |
| Board drag and drop | Native HTML5 drag and drop, plus a status menu on each board card as the keyboard and touch alternative | No dependency for a one-axis drop. |
| Fonts | Geist and Geist Mono for the UI, JetBrains Mono, Fira Code and Source Code Pro for the terminals, **self-hosted** from their `@fontsource-variable` npm packages | The prototype loads Google Fonts, which breaks "offline" and "no telemetry". |
| Lint and format | Biome, its recommended rules plus Foreman's own as GritQL plugins in `ui/lint/` | One tool, fast, replaces ESLint and Prettier. A plugin catches a mistake the recommended rules miss: `effect-block-body.grit` wants every `useEffect` callback in braces, since an expression body returns its value to React as the cleanup (Chrome's scroll methods return a Promise now, and an effect that only scrolled crashed the Board on 2026-11-02). |

## Agent images

Foreman builds the images its containers run, itself, through Podman's build API. They form **one chain of four layers**, each extending the one above. A layer with no Containerfile of its own is **transparent**: Foreman simply tags the parent image with the layer's name (no Containerfile, no build), so the chain always resolves and every project always has an image to run.

| # | Image name | Defined by | Contents |
|---|---|---|---|
| 1 | `localhost/foreman/base` | Containerfile shipped in the binary | OS, `git`, `wiki`, common shell tools (curl, jq, make, ssh client, less, procps…), the non-root user |
| 2 | `localhost/foreman/harness` | Containerfile shipped in the binary | Every supported harness CLI (v1: Claude Code, Pi and Codex CLI; Hermes Agent if it fits), installed last so a harness update reuses everything above |
| 3 | `localhost/foreman/<home>` | the CEO's own Containerfile, machine-wide (where it lives is [open](#open-questions)), else transparent | The CEO's own tools for every project on this machine |
| 4 | `localhost/foreman/<home>/<project>` | a Containerfile in the project repo (where exactly is [open](#open-questions)), else transparent | The project's toolchain, so `checks` can run (node, cargo, terraform…) |

Every instance of a project runs layer 4, so the project's image name is stable and predictable. One harness layer holds every harness, rather than one image per harness, so layers 3 and 4 exist once per home and project, not once per harness.

**Phasing.** The chain above is the target. Phase 1 ([Terminal multiplexer](../../backlog/epics/terminal-multiplexer.md)) builds **one image for every project**, from a Containerfile that starts from [`pclaude`](https://github.com/brickpop/pclaude)'s Dockerfile (commit `7c9f9079`): a **basic Debian box** (`node:26-slim`, so Node; plus Bun, Go, Python 3 (Debian's `python3`, added 2026-11-12) with [uv](https://docs.astral.sh/uv/) (copied from Astral's `ghcr.io/astral-sh/uv` image, added 2026-11-13; no pip or venv, the CEO: only uv), `just`, the `wiki` CLI, git, curl, jq, make and the usual shell tools), then the harnesses installed last, Claude Code, Pi (`@earendil-works/pi-coding-agent`, added 2026-10-18) and Codex (`@openai/codex`, added 2026-11-06; a changed Containerfile is a new image, built at the next start); `ENTRYPOINT []`, `CMD ["claude"]`. Anything more specialised comes from the global or project overrides, later. `foreman-agent` isn't in the image: Foreman mounts it into each container ([Team tools → Transport](./team-tools.md#transport)). The file is `images/Containerfile` and the image is named `localhost/foreman/harness`, the target design's name for the harness layer, so the full chain builds on it later. The separate harness layer (Hermes), the global and project overrides, Refresh with prune, Prune and the full Maintenance screen come later ([Full image chain](../../backlog/image-chain.md)).

**Names.** `<home>` is a short hash of the home's path (as in container names, [Agent runtime → Recovery](./agent-runtime.md#recovery)) and `<project>` the project's name, slugified to what an image name allows (lowercase `a-z0-9`, with `.`, `_` or `-` between). Project names are unique in any case, so two never slugify to the same name. The hierarchy in the name mirrors the chain, so `podman images localhost/foreman/<home>` shows one home's images at a glance, and several homes on one machine (the CEO's, `just dev`'s, a test's) never collide.

**Overrides.** An override is a plain Containerfile the CEO writes in their editor; Foreman never generates or templates one. Foreman always builds it with the parent image passed as the `BASE` build argument, so an override starts with:

```dockerfile
ARG BASE
FROM ${BASE}
# the CEO's or the project's tools, e.g.:
RUN npm install -g pnpm
```

What is on disk is exactly what is built, so `podman build --build-arg BASE=localhost/foreman/<home> .` reproduces it by hand.

**Builds.**
- A transparent layer is never built: Foreman just tags the parent image with the layer's name (`podman tag`), and moves the tag when the parent changes.
- Each built layer is tagged with a hash of its inputs (its Containerfile, its build context and its parent's image ID), so it is rebuilt only when something it depends on changed, and rebuilding a parent invalidates its children. Builds run on first use and when the file watcher sees a Containerfile appear, change or disappear.
- Harness CLIs, Go and the wiki CLI update often (the image takes their latest release), which no input hash can notice. **Refresh** on layer 1 or 2 rebuilds it with `--pull --no-cache` (the latest base image and tools) and then rebuilds the layers below it. Running sessions keep their image; new and restarted sessions use the refreshed one.
- **Prune** removes Foreman's outdated images and their build cache: anything labelled `foreman.image` that no current layer and no running container uses. It never touches other images in the CEO's Podman store. Refresh offers it as an option; it is also available on its own, showing the space it would reclaim.
- A deployment whose image is building keeps its instances `starting`. A failed build raises a **blocked** [request](../features/requests.md) carrying the build log.
- Images never contain secrets. Credentials are injected at run time ([Session runner](./session-runner.md#credentials)).

## Wire formats

| Channel | Format | Why |
|---|---|---|
| PTY streams (`/ws/pty/:instance`) | WebSocket **binary frames carrying raw PTY bytes**, untouched; a small JSON text frame for control (resize, attach-from-offset) | xterm.js consumes raw bytes. Any envelope (JSON, protobuf) adds encode/decode on the hottest path for nothing. |
| API (`/api/*`) | JSON over HTTP | Debuggable with curl, no codegen at runtime. |
| Events (`/api/events`) | SSE, one JSON object per event | Browser-native reconnect. |
| Team tools | MCP (JSON-RPC), as the harnesses require | Not ours to choose. |

The Go structs are the single source of truth for API and event shapes. TypeScript types are generated from them with `tygo` at build time, so the UI can't drift from the server. Protobuf was considered and rejected: the only high-volume traffic is the PTY stream, which is already raw binary frames, and since server and UI ship in one binary there are no independently versioned clients to protect.

## Repo layout

One Go module at the root, the web UI in `ui/`:

```
foreman/
├── cmd/
│   ├── foreman/          # the server binary, and the composition root
│   ├── foreman-agent/    # the binary shipped into every container (stdio MCP server, hook command)
│   └── fakeharness/      # scripted harness for tests (prints, waits, calls team tools)
├── internal/             # one package per building block
│   ├── runner/           # Execution engine: Runner interface + podman/ and direct/ (each with its image builder)
│   ├── terminal/         # Terminal: one live process, its buffer, viewers, snapshots
│   ├── screen/           # the Terminal's server-side terminal state (Screen)
│   ├── sessionlog/       # the Terminal's raw byte log (history)
│   ├── harness/          # Harness adapters: Claude Code, Pi, Hermes, custom, fake
│   ├── prompt/           # session prompt composer (used by the harness adapters)
│   ├── agentlink/        # Agent link: the agent port, tokens, tool calls and hook events
│   ├── instances/        # Instance manager: what runs, status, restarts, queue
│   ├── messaging/        # Messaging: queues and deliveries between agents and the CEO
│   ├── workflow/         # Workflow: requests, work items, merge requests, gates
│   ├── git/              # Git: worktrees per agent, branch policy, pushes
│   ├── forge/            # Forge adapters: github, gitea (also Forgejo), none
│   ├── store/            # State store: SQLite, migrations
│   ├── home/             # Home & config: projects (records and repos), teams, settings, file watcher
│   ├── wiki/             # thin layer over agentic-wiki: tree, entries with their lookup tables, images
│   ├── events/           # in-process event bus
│   ├── cos/              # Chief of Staff
│   ├── server/           # Web server: HTTP, SSE, WebSocket handlers (depends only on interfaces)
│   └── access/           # who may talk to the server: the access-token guard
├── images/               # Containerfiles for the base and harness layers (embedded)
├── ui/                   # Vite + React app (src/, built to ui/dist/app) and the Go package that embeds it
├── wiki/                 # this knowledge base
├── justfile
└── README.md
```

The packages map one-to-one to the [building blocks](./building-blocks.md), so "where does X live" has one answer.

## Build, run and test

All through `just` (`just help` lists everything):

- `just setup`: installs the UI's dependencies (`bun install` in `ui/`). Vite, Tailwind, TypeScript 7 and Biome ship native binaries per platform, so each OS needs its own checkout of the repo (e.g. a git worktree per machine) with its own `ui/node_modules`.
- `just dev [real|fake] [home]`: rebuilds the UI on every change (`vite build --watch`) and runs the Go server on a scratch home (`--home`), serving the UI from disk with the development flag `--ui-dir`. Each harness has its own home, so each is always on the engine it needs, and runs beside the CEO's own Foreman (one per home): `just dev` runs Claude Code on Podman in `.dev/real`, where its login stays; `just dev fake` runs agents on the fake harness (`--fake-harness`) on the Direct engine in `.dev/fake`, for staging cases Claude Code won't produce on cue (a crash, a flood of output) or working on the UI without Podman or tokens. A new home starts with team `dev-team` (Tic leads, Tac and Toe) and project `DevProject` (one code repo, `some-app`, in `.dev/<harness>-repos`; the project's record is made through the API once the server answers), ready to deploy. A home given explicitly must already be on the engine its harness needs; otherwise the recipe stops with a clear message. There is no separate Vite dev server: one origin keeps the access token, Host and Origin checks exactly as in production. Reload the page to see a UI change.
- `just build`: the release build. `just agent` (foreman-agent for Linux on amd64 and arm64, cross-compiled by Go alone: no Podman needed on the build machine), `bun run build` in `ui/`, then `go build -tags release`, giving one binary in `bin/foreman` with the UI, the agent image's Containerfile and both foreman-agents embedded. The `release` tag makes the compiler refuse a build missing any of them, and the release tests check what is embedded (each foreman-agent is a static Linux program for its CPU type; every file `index.html` loads is there). A build without the tag (`just dev`, tests, tools) embeds whatever is built, so a fresh checkout still compiles; it then says at startup that agents start without the team tools.
- `just test`: the Go tests (and the UI unit tests once there are any). Go commands name their packages explicitly (`./cmd/... ./internal/... ./ui`), so stray `.go` files elsewhere are never picked up.
- `just test-claude [profile]`: a real Claude Code through Foreman's whole stack and API (Podman, and the CEO's own login in that profile, `default` unless another is given, never copied): the session starts with no dialog, a typed line is sent once, the hooks give running, idle and awaiting, and an ask wakes a teammate who reads it and replies. About 30 seconds; spends tokens. Run it after a Claude Code update.
- `just e2e` (not yet; added with the first UI flows): Playwright against a real `foreman` binary using the Direct engine and the fake harness. The [use cases](../use-cases.md) are the e2e suite.
- `just lint`: `gofmt`, `go vet`, `staticcheck` (pinned as a Go tool in `go.mod`, run with `go tool staticcheck`), Biome.
- `just check`: the wiki health check (`wiki --root wiki check`). Other wiki commands run directly: `wiki --root wiki <command>` from the project root.

Testing approach:
- **Go:** standard `testing`. Components are tested through their real interfaces: a real SQLite in a temp dir, a real git repo in a temp dir, the Direct engine with `fakeharness`. No mocks of things we own.
- **The fake harness** is the key test tool: a scripted program that prints output (including ANSI), waits for stdin, goes idle, and calls the team tools. The Terminal, Instance manager, Messaging and Workflow are then testable end to end, deterministically, without models or network.
- **Podman runner:** integration tests behind a build tag, run when Podman is present.
- **Web:** Vitest + Testing Library for components with logic (board, terminal pane wiring, request panels); Playwright for flows.

Runtime prerequisites on the CEO's machine: `git`, `podman` (with `podman machine` on macOS), and the harness accounts. The README will cover setup in a few steps.

## Open questions

1. **The global and project image layers.** Layer 3 was a workspace's own Containerfile (`<workspace>/.foreman/Containerfile`) and layer 4 one in each project folder; with the workspace gone (2026-10-20), neither has a home yet. Options: layer 3 in the global repo (`~/.foreman/config/Containerfile`, versioned with the teams and settings) and layer 4 in the project repo (`~/.foreman/projects/<name>/Containerfile`, travelling with the project); or drop layer 3. Recommendation: the global repo and the project repo, decided with [Full image chain](../../backlog/image-chain.md).
