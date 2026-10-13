---
type: spec
status: planned
tags: [architecture, runtime, ui]
---

# Tech stack

The languages, libraries, repo layout, build and test setup. This entry is the single source of truth for "what do we use": other entries link here instead of naming libraries. Each choice says **why**, so that it can be challenged later. Choices marked *proposed* wait on the [open questions](#open-questions) below; the rest are decided.

Guiding constraints, from the [vision](../vision.md): one local binary, local-first and offline-capable (no telemetry, no CDN calls), 20+ concurrent sessions on a laptop, macOS and Linux first, as few dependencies as possible, and every moving part behind a small swappable interface.

## Decided

| Decision | Why |
|---|---|
| **Go** for the server | The `wiki` engine ([Wiki engine](./wiki-engine.md)) is a Go module with public packages, so Foreman reuses its rules instead of reimplementing them. PTY handling, process supervision and a single static binary are Go's home ground, and the PTY proofs of concept were already in Go. |
| **One binary** serving the API and the built SPA | PRD requirement. The SPA is embedded with `//go:embed`. |
| **Every agent instance runs in a rootless Podman container** | Real isolation from day one: an agent sees only what is mounted ([Session runner](./session-runner.md), [Security](./security.md)). |
| **A swappable `Runner` interface** for where sessions run | Podman in production, a local PTY runner for hermetic tests and development. tmux or anything else can be added later without touching the rest ([Session runner](./session-runner.md)). |
| **Foreman builds the agent images, in four layers** | Base, harness, the CEO's workspace extras, the project toolchain ([Agent images](#agent-images)). |
| **Tailwind 4 + unstyled Radix primitives** for the UI | The prototype's tokens become CSS variables, Tailwind utilities sit over them, and Radix brings focus traps, Escape and keyboard navigation. |
| **JSON API typed from Go structs (`tygo`); PTY as raw binary frames** | See [Wire formats](#wire-formats). Protobuf adds nothing on the PTY path and solves a versioning problem we don't have. |
| **A per-install access token on every request**, delivered via the URL Foreman prints and opens at start | Listening on 127.0.0.1 doesn't stop websites in the browser or the agents' containers from reaching the API ([Security](./security.md#access-token-decided)). |

## Server (Go)

| Concern | Choice | Why |
|---|---|---|
| Go version | The current stable release (1.27 at the time of writing) | Tracks stable; nothing needs a pinned older version. |
| HTTP routing | stdlib `net/http` (`ServeMux` method and path patterns) | Enough for ~30 endpoints; no framework. |
| Server-sent events | stdlib (`http.Flusher`) | A dozen lines; no library needed. |
| WebSocket (PTY streams) | `github.com/coder/websocket` | Small, context-aware, maintained; the stdlib has no WebSocket. |
| Local PTY (test/dev runner) | `github.com/creack/pty` | The standard Go PTY package; used by the POCs. |
| Podman | The libpod REST API over the Podman socket, with plain `net/http` (*proposed*) | Avoids the very heavy `containers/podman/v5/pkg/bindings` module. See [Session runner](./session-runner.md#open-questions). |
| ACP (agent tools) | MCP server via `github.com/modelcontextprotocol/go-sdk` (official SDK), Streamable HTTP transport | Claude Code and Codex speak MCP natively ([ACP](./acp.md)). |
| Runtime state | SQLite via `modernc.org/sqlite` (pure Go) | No CGO, so the binary cross-compiles and stays static. WAL mode. |
| Queries and migrations | Hand-written `database/sql` in one `store` package; numbered `.sql` migrations embedded and applied by `PRAGMA user_version` | About 8 tables ([Persistence](./persistence.md)). An ORM or a code generator would cost more than it saves. |
| YAML (project, team, settings files) | `go.yaml.in/yaml/v3` (the maintained continuation of `gopkg.in/yaml.v3`) | Its `Node` API edits a team file in place while keeping the CEO's comments and order. |
| TOML (`wiki.toml`) | Through the wiki engine (`bundle.DecodeTool`) | Never parsed twice. |
| Wiki | `github.com/agentic-wiki/wiki` (`bundle`, `index`, `parse`) | See [Wiki engine](./wiki-engine.md). |
| Markdown → HTML | `github.com/yuin/goldmark` on the server, raw HTML disabled (*proposed*) | Links are resolved by the engine's resolver and checkboxes carry their source line, so the browser never re-implements a wiki rule. Wiki content is written by agents, so it is untrusted: no raw HTML, and URLs are sanitised ([Security](./security.md)). |
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
| Routing | TanStack Router (*proposed*) | Typed path and search params (project / tab / entry path, team / tab, sessions / deployment). |
| Styling | Tailwind CSS 4 with the [design tokens](../design/design-system.md) declared as CSS variables | The prototype is a token system with inline styles, which maps 1:1 onto Tailwind utilities over the same variables. Both themes switch by `data-theme` on `<html>`. |
| Accessible primitives | Radix UI primitives (dialog, popover, tabs, toggle), styled with our tokens | They bring what the prototype lacks: focus traps, Escape to close, real disabled states, keyboard navigation. Their default look is not used. |
| Terminal | `@xterm/xterm` with the fit, web-links and serialize addons | Real ANSI terminal, resize, clickable links; "serialize" backs the Copy button. |
| Board drag and drop | Native HTML5 drag and drop, plus a status menu on each card as the keyboard and touch alternative (*proposed*) | No dependency for a one-axis drop. |
| Fonts | Geist and Geist Mono, **self-hosted** from the `geist` npm package | The prototype loads Google Fonts, which breaks "offline" and "no telemetry". |
| Avatars | A React SVG component ported from the [Avatar](../design/avatar.md) spec | No image assets. |
| Lint and format | Biome | One tool, fast, replaces ESLint and Prettier. |

## Agent images

Foreman builds the images its containers run, itself, through Podman's build API. Images are **layered**, and each layer is tagged with a hash of its inputs (Containerfile + context + parent tag), so a layer is rebuilt only when something it depends on changes:

| Layer | Defined by | Contents |
|---|---|---|
| 1. `foreman-base` | Containerfile shipped in the binary | OS, `git`, common CLI tools, the non-root user |
| 2. Harness | Containerfile per harness, shipped in the binary | Claude Code, Codex CLI, Pi or Hermes Agent |
| 3. Workspace (optional) | `~/agents-ws/.foreman/Containerfile` | The CEO's custom tools for every project |
| 4. Project (optional) | `<project>/.foreman/Containerfile` | The project's toolchain, so `checks` can run (node, cargo, terraform…) |

Each optional layer starts `FROM` an argument Foreman fills with the parent tag, so it doesn't hard-code the chain. An instance runs the most specific image for its (harness, project).

- Builds run on first use and whenever an input changes (the file watcher sees Containerfile edits). Status and build logs are visible in the [settings screen](../features/settings.md), with a **Rebuild** action.
- A deployment whose image is building keeps its instances `starting`. A failed build raises a **blocked** [request](../features/requests.md) carrying the build log.
- Images never contain secrets. Harness credentials are injected at run time ([Session runner](./session-runner.md#open-questions), [Harness adapters](./harness-adapters.md)).

## Wire formats

| Channel | Format | Why |
|---|---|---|
| PTY streams (`/ws/pty/:instance`) | WebSocket **binary frames carrying raw PTY bytes**, untouched; a small JSON text frame for control (resize, attach-from-offset) | xterm.js consumes raw bytes. Any envelope (JSON, protobuf) adds encode/decode on the hottest path for nothing. |
| API (`/api/*`) | JSON over HTTP | Debuggable with curl, no codegen at runtime. |
| Events (`/api/events`) | SSE, one JSON object per event | Browser-native reconnect. |
| ACP | MCP (JSON-RPC), as the harnesses require | Not ours to choose. |

The Go structs are the single source of truth for API and event shapes. TypeScript types are generated from them with `tygo` at build time, so the UI can't drift from the server. Protobuf was considered and rejected: the only high-volume traffic is the PTY stream, which is already raw binary frames, and since server and UI ship in one binary there are no independently versioned clients to protect.

## Repo layout

One Go module at the root, one web app in `web/`:

```
foreman/
├── cmd/
│   ├── foreman/          # the server binary
│   └── fakeharness/      # scripted harness for tests (prints, waits, calls ACP tools)
├── internal/             # one package per component of the architecture overview
│   ├── server/           # HTTP, SSE, WebSocket handlers; embeds web/dist
│   ├── workspace/        # indexer, config loading (project, team, settings), file watcher
│   ├── runtime/          # supervisor: instances, lifecycle, concurrency queue
│   ├── runner/           # Runner interface + podman/ and pty/
│   ├── harness/          # per-harness adapters
│   ├── sandbox/          # clones, mirrors, branch policy
│   ├── acp/              # MCP server and message router
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
- **The fake harness** is the key test tool: a scripted program that prints output (including ANSI), waits for stdin, goes idle, and calls the ACP tools. Supervisor, ACP routing, requests and the pipeline are then testable end to end, deterministically, without models or network.
- **Podman runner:** integration tests behind a build tag, run when Podman is present.
- **Web:** Vitest + Testing Library for components with logic (board, terminal pane wiring, request cards); Playwright for flows.

Runtime prerequisites on the CEO's machine: `git`, `podman` (with `podman machine` on macOS), and the harness accounts. The README will cover setup in a few steps.

## Open questions

1. **Router.** TanStack Router (recommended: typed params, pairs with TanStack Query) · React Router.
2. **Markdown rendering on the server.** goldmark on the server, with the engine resolving links (recommended: one link resolver, and untrusted content is sanitised in one place) · render in the browser (markdown-it or similar), which would need a second link resolver and its own sanitiser.
3. **Harness layer rebuilds.** Harness CLIs update often. Does Foreman rebuild the harness layer on a schedule, on a "check for updates" action, or only when the CEO presses Rebuild? (Recommended: Rebuild, plus a "harness version behind" hint in Settings.)
4. **Board drag and drop.** Native HTML5 plus a status menu (recommended: no dependency) · `@dnd-kit` (smoother, touch support, one more dependency).
