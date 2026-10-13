---
type: spec
status: planned
tags: [architecture, runtime, ui]
---

# Tech stack

The languages, libraries, repo layout, build and test setup. This entry is the single source of truth for "what do we use": other entries link here instead of naming libraries. Each choice says **why**, so that it can be challenged later. Every choice here is decided; the [walking skeleton](../../backlog/walking-skeleton.md) confirms the runtime ones.

Guiding constraints, from the [vision](../vision.md): one local binary, local-first and offline-capable (no telemetry, no CDN calls), 20+ concurrent sessions on a laptop, macOS and Linux first, as few dependencies as possible, and every moving part behind a small swappable interface.

## Decided

| Decision | Why |
|---|---|
| **Go** for the server | The `wiki` engine ([Wiki engine](./wiki-engine.md)) is a Go module with public packages, so Foreman reuses its rules instead of reimplementing them. PTY handling, process supervision and a single static binary are Go's home ground, and the PTY proofs of concept were already in Go. |
| **One binary** serving the API and the built SPA | PRD requirement. The SPA is embedded with `//go:embed`. |
| **Every agent instance runs in a rootless Podman container** | Real isolation from day one: an agent sees only what is mounted ([Session runner](./session-runner.md), [Security](./security.md)). |
| **A swappable `Runner` interface** for where sessions run | Podman in production, a local PTY runner for hermetic tests and development. tmux or anything else can be added later without touching the rest ([Session runner](./session-runner.md)). |
| **Foreman builds the agent images as a four-layer chain** | base → harness → workspace → project, a layer without a Containerfile is just a tag of its parent; Refresh and Prune live in Settings → Maintenance ([Agent images](#agent-images)). |
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
| Local PTY (test/dev runner) | `github.com/creack/pty` | The standard Go PTY package; used by the POCs. |
| Podman | The libpod REST API over the Podman socket, with plain `net/http` | Avoids the very heavy `containers/podman/v5/pkg/bindings` module. See [Session runner](./session-runner.md#podman-runner-production). |
| Team tools (agent tools) | MCP server via `github.com/modelcontextprotocol/go-sdk` (official SDK), Streamable HTTP transport | Claude Code and Codex speak MCP natively ([Team tools](./team-tools.md)). |
| Runtime state | SQLite via `modernc.org/sqlite` (pure Go) | No CGO, so the binary cross-compiles and stays static. WAL mode. |
| Queries and migrations | Hand-written `database/sql` in one `store` package; numbered `.sql` migrations embedded and applied by `PRAGMA user_version` | About 8 tables ([Persistence](./persistence.md)). An ORM or a code generator would cost more than it saves. |
| YAML (project, team, settings files) | `go.yaml.in/yaml/v3` (the maintained continuation of `gopkg.in/yaml.v3`) | Its `Node` API edits a team file in place while keeping the CEO's comments and order. |
| TOML (`wiki.toml`) | Through the wiki engine (`bundle.DecodeTool`) | Never parsed twice. |
| Wiki | `github.com/agentic-wiki/wiki` (`bundle`, `index`, `parse`) | See [Wiki engine](./wiki-engine.md). |
| Markdown → HTML | `github.com/yuin/goldmark` on the server, raw HTML disabled | Links are resolved by the engine's resolver and checkboxes carry their source line, so the browser never re-implements a wiki rule. Wiki content is written by agents, so it is untrusted: no raw HTML, and URLs are sanitised ([Security](./security.md)). |
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
| Package manager and scripts | Bun | Same as the CEO's other projects. Only for the `web/` toolchain; nothing runs on Bun in production. |
| Server state | TanStack Query | Caching, refetching and optimistic updates (the board drag). SSE events invalidate or patch the cached queries, so there is one data path. |
| Routing | TanStack Router | Typed path and search params (project / tab / entry path, team / tab, sessions / deployment). |
| Styling | Tailwind CSS 4 with the [design tokens](../design/design-system.md) declared as CSS variables | The prototype is a token system with inline styles, which maps 1:1 onto Tailwind utilities over the same variables. Both themes switch by `data-theme` on `<html>`. |
| Accessible primitives | Radix UI primitives (dialog, popover, tabs, toggle), styled with our tokens | They bring what the prototype lacks: focus traps, Escape to close, real disabled states, keyboard navigation. Their default look is not used. |
| Terminal | `@xterm/xterm` with the fit, web-links and serialize addons | Real ANSI terminal, resize, clickable links; "serialize" backs the Copy button. |
| Board drag and drop | Native HTML5 drag and drop, plus a status menu on each board card as the keyboard and touch alternative | No dependency for a one-axis drop. |
| Fonts | Geist and Geist Mono, **self-hosted** from the `geist` npm package | The prototype loads Google Fonts, which breaks "offline" and "no telemetry". |
| Avatars | A React SVG component ported from the [Avatar](../design/avatar.md) spec | No image assets. |
| Lint and format | Biome | One tool, fast, replaces ESLint and Prettier. |

## Agent images

Foreman builds the images its containers run, itself, through Podman's build API. They form **one chain of four layers**, each extending the one above. A layer with no Containerfile of its own is **transparent**: Foreman simply tags the parent image with the layer's name (no Containerfile, no build), so the chain always resolves and every project always has an image to run.

| # | Image name | Defined by | Contents |
|---|---|---|---|
| 1 | `localhost/foreman/base` | Containerfile shipped in the binary | OS, `git`, `wiki`, common shell tools (curl, jq, make, ssh client, less, procps…), the non-root user |
| 2 | `localhost/foreman/harness` | Containerfile shipped in the binary | Every supported harness CLI (v1: Claude Code and Pi, Hermes Agent if it fits; Codex CLI later), installed last so a harness update reuses everything above |
| 3 | `localhost/foreman/ws/<workspace>` | `~/agents-ws/.foreman/Containerfile`, else transparent | The CEO's own tools for every project in this workspace |
| 4 | `localhost/foreman/ws/<workspace>/<project>` | `<project>/.foreman/Containerfile`, else transparent | The project's toolchain, so `checks` can run (node, cargo, terraform…) |

Every instance of a project runs layer 4, so the project's image name is stable and predictable. One harness layer holds every harness, rather than one image per harness, so layers 3 and 4 exist once per workspace and project, not once per harness.

**Names.** `<workspace>` is the workspace folder name and `<project>` the project folder name, slugified to what an image name allows (lowercase `a-z0-9`, with `.`, `_` or `-` between). Two folders that slugify to the same name are reported as a configuration error. The hierarchy in the name mirrors the chain, so `podman images localhost/foreman/ws/agents-ws` shows a workspace at a glance, and several workspaces on one machine never collide.

**Overrides.** An override is a plain Containerfile the CEO writes in their editor; Foreman never generates or templates one. Foreman always builds it with the parent image passed as the `BASE` build argument, so an override starts with:

```dockerfile
ARG BASE
FROM ${BASE}
# the workspace's or the project's tools, e.g.:
RUN npm install -g pnpm
```

What is on disk is exactly what is built, so `podman build --build-arg BASE=localhost/foreman/ws/agents-ws .` reproduces it by hand.

**Builds.**
- A transparent layer is never built: Foreman just tags the parent image with the layer's name (`podman tag`), and moves the tag when the parent changes.
- Each built layer is tagged with a hash of its inputs (its Containerfile, its build context and its parent's image ID), so it is rebuilt only when something it depends on changed, and rebuilding a parent invalidates its children. Builds run on first use and when the file watcher sees a Containerfile appear, change or disappear.
- Harness CLIs update often, which no input hash can notice. **Refresh** on layer 1 or 2 rebuilds it with `--pull --no-cache` (the latest base image and harness CLIs) and then rebuilds the layers below it. Running sessions keep their image; new and restarted sessions use the refreshed one.
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

One Go module at the root, one web app in `web/`:

```
foreman/
├── cmd/
│   ├── foreman/          # the server binary
│   └── fakeharness/      # scripted harness for tests (prints, waits, calls team tools)
├── internal/             # one package per component of the architecture overview
│   ├── server/           # HTTP, SSE, WebSocket handlers; embeds web/dist
│   ├── workspace/        # indexer, config loading (project, team, settings), file watcher
│   ├── runtime/          # supervisor: instances, lifecycle, concurrency queue
│   ├── runner/           # Runner interface + podman/ and pty/
│   ├── harness/          # per-harness adapters
│   ├── sandbox/          # clones, mirrors, branch policy
│   ├── teamtools/        # MCP server and message router
│   ├── requests/         # request engine
│   ├── pipeline/         # pipeline tracker
│   ├── forge/            # github, gitea (also Forgejo), none
│   ├── wiki/             # thin layer over agentic-wiki + markdown rendering
│   ├── prompt/           # session prompt composer
│   ├── cos/              # Chief of Staff
│   └── store/            # SQLite, migrations
├── images/               # Containerfiles per harness (embedded)
├── web/                  # Vite + React app (src/, built to web/dist)
├── wiki/                 # this knowledge base
├── justfile
└── README.md
```

The packages map one-to-one to the components in the [architecture overview](./overview.md), so "where does X live" has one answer.

## Build, run and test

All through `just` (`just help` lists everything):

- `just dev`: the Go server (with the PTY runner unless Podman is available) and the Vite dev server, which proxies `/api` and `/ws` to it.
- `just build`: `bun run build` in `web/`, then `go build` with the SPA embedded, giving one `foreman` binary.
- `just test`: `go test ./...` and the web unit tests.
- `just e2e`: Playwright against a real `foreman` binary using the PTY runner and the fake harness. The [use cases](../use-cases.md) are the e2e suite.
- `just lint`: `go vet`, `staticcheck`, Biome.
- `just wiki <args>`: `wiki --root wiki <args>`, and `just check` runs the wiki health check with the rest.

Testing approach:
- **Go:** standard `testing`. Components are tested through their real interfaces: a real SQLite in a temp dir, a real git repo in a temp dir, the PTY runner with `fakeharness`. No mocks of things we own.
- **The fake harness** is the key test tool: a scripted program that prints output (including ANSI), waits for stdin, goes idle, and calls the team tools. Supervisor, team-message routing, requests and the pipeline are then testable end to end, deterministically, without models or network.
- **Podman runner:** integration tests behind a build tag, run when Podman is present.
- **Web:** Vitest + Testing Library for components with logic (board, terminal pane wiring, request panels); Playwright for flows.

Runtime prerequisites on the CEO's machine: `git`, `podman` (with `podman machine` on macOS), and the harness accounts. The README will cover setup in a few steps.
