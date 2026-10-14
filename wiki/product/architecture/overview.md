---
type: spec
status: planned
tags: [architecture, runtime, sessions, team-tools, api]
---

# Architecture overview

Foreman is **one Go binary** that serves the API and the built SPA, supervises one containerised harness process per agent [instance](../models/instance.md), and routes everything that crosses between agents or reaches the CEO. Languages, libraries and repo layout are in [Tech stack](./tech-stack.md). This entry names the components and how they connect; each has its own entry.

It listens on `localhost:8888` by default ([Workspace settings](../models/workspace-settings.md) can switch it to the local network, see [Security](./security.md)).

## Components

| # | Component | Job | Entry |
|---|---|---|---|
| 1 | **HTTP server + SPA** | Serves the JSON API, SSE, the PTY WebSockets and the embedded UI build | [API](./api.md) |
| 2 | **Workspace indexer** | Scans the [workspace](../models/workspace.md) for projects and repos. Watches config, team files and wikis; pushes changes to clients within ~1 s | [Persistence](./persistence.md) |
| 3 | **Session supervisor** | Starts harnesses through the [session runner](./session-runner.md), one per instance. Owns lifecycle, restarts, the concurrency queue and logs. Exposes PTY I/O over WebSocket | [Agent runtime](./agent-runtime.md) |
| 4 | **Sandbox manager** | Creates and refreshes per-instance clones, checks out branches, enforces branch policy | [Sandboxes and git policy](./sandboxes-and-git.md) |
| 5 | **Team-tools router** | Exposes the agent tools to harnesses (MCP), routes messages between instances of one deployment, stores them, injects replies into the requester's session | [Team tools](./team-tools.md) |
| 6 | **Request engine** | Creates, deduplicates and resolves [requests](../features/requests.md) (agent-raised and Foreman-raised). Resolving one writes the answer back to the agent and resumes it | [Request](../models/request.md) |
| 7 | **Pipeline tracker** | Tracks work items through their stages and the pull requests that carry them; drives forge adapters and notices merges (on the forge, or in the project repo without one) | [Pipeline and PRs](../features/pipeline-and-prs.md), [Work item](../models/work-item.md), [Pull request](../models/pull-request.md) |
| 8 | **Forge adapters** | GitHub, Forgejo, Gitea (REST) and `none` (local). Push lead branches, open PRs, poll their state and checks, comment on send-back, close discarded ones. Never merge | [Forge adapters](./forge-adapters.md) |
| 9 | **Wiki engine** | Parses the bundle, builds tree, entries, boards, links; atomic single-field writes. Imports the agentic-wiki engine | [Wiki engine](./wiki-engine.md) |
| 10 | **Prompt composer** | Builds each instance's session prompt from soul + team + deployment + project | [Session prompt](./session-prompt.md) |
| 11 | **Notifier** | Pushes events to the UI (SSE); the UI raises browser notifications and toasts | [API](./api.md), [Notifications](../features/notifications.md) |
| 12 | **Chief of Staff** | A Foreman-owned harness session with tools to read and edit Foreman files and project config (on a branch), create teams, hire agents and answer free-form requests | [Chief of Staff](../features/chief-of-staff.md) |

## How they connect

```
 Browser (SPA) ──HTTP/JSON, SSE, WS──▶ HTTP server ──▶ request engine · pipeline · wiki engine · indexer
                                            │
                                            ▼
                                     session supervisor ──▶ session runner (Podman | PTY) ──▶ harness in container
                                            ▲                                                   │
                                            │                       MCP (team-tools listener, token)   │
                                        team-tools router ◀────────────────────────────────────────────┘
                                            │
                              sandbox manager · forge adapters · state.db
```

- The UI never talks to a harness directly: PTY bytes flow through the supervisor, which also tees them to the session log.
- Harnesses never talk to each other directly: only the team-tools router moves `output` between instances, and only within one [deployment](../models/deployment.md).
- Harnesses never touch a forge, a secret or another sandbox: pushes to forges are done by Foreman ([Sandboxes and git policy](./sandboxes-and-git.md)).

## Modularity

**Interfaces everywhere between components.** Every primitive must be swappable with ease, at any time: the way sessions run, how a harness is driven, where secrets live, which forge is used, how state is stored, how events travel. These rules make that true in the code, and reviews check them:

1. **One package per component, one small interface per seam.** A component exposes only what others may ask of it. Consumers depend on that interface (declared where it is consumed, Go-style), never on a concrete type.
2. **One composition root.** `cmd/foreman` builds the concrete implementations and wires them together. No package constructs its own dependencies; no globals, no init-time singletons. Swapping an implementation is a change in one place.
3. **No reaching across.** Components talk through their interfaces or the in-process event bus, never through each other's internals. Each component owns its own tables in `state.db`; no SQL touches another component's tables.
4. **Implementation types stay home.** Data crossing a seam is plain Go structs (the same ones the UI's TypeScript types are generated from). Nothing like a libpod type, a GitHub API type or an xterm-specific frame leaks outside its implementation package.
5. **Every boundary is an interface, even with a single implementation today.** Swappability is the requirement, not a reaction to a second implementation showing up: any piece can be replaced later without touching its consumers, and every consumer can be tested against a double. The "no abstractions without clear need" rule in `AGENTS.md` applies *inside* a component, whose internals stay plain code, not to the boundaries between components.

The seams known today (every new component adds its own):

| Seam | Package | Implementations |
|---|---|---|
| Session runner: where a harness process runs | `runner` | Podman (production), local PTY (tests, dev); tmux possible later ([Session runner](./session-runner.md)) |
| Session driver: how Foreman talks to a harness | `harness` | PTY (v1); Agent Client Protocol later ([Harness adapters](./harness-adapters.md#session-drivers)) |
| Harness adapter: per-harness details | `harness` | Claude Code, Pi, Hermes Agent, custom; Codex later; the fake harness in tests |
| Image builder | `runner/podman` | libpod build API; a no-op for the PTY runner ([Agent images](./tech-stack.md#agent-images)) |
| Forge | `forge` | GitHub, Gitea (also Forgejo), none; a fake forge server in tests ([Forge adapters](./forge-adapters.md)) |
| Secret store | `secrets` | 0600 file (v1); OS keychain later ([Secrets](../models/secrets.md)) |
| Wiki engine | `wiki` | the agentic-wiki engine ([Wiki engine](./wiki-engine.md)) |
| Runtime state | `store` | SQLite (also in tests, in a temp dir) ([Persistence](./persistence.md)) |
| Access guard: who may reach the server | `access` | the per-install token guard ([Security](./security.md#access-token)); real user auth if ever needed |
| Workspace, as the server sees it | `workspace` | the folder on disk |
| UI files | `ui` | embedded build, or a folder on disk (`--ui-dir`) |

## Process model

- **One container per running instance**, decided: every harness runs in a rootless Podman container with only its own sandbox and the wiki mounted ([Session runner](./session-runner.md)). Inside it the harness is attached to a TTY.
- Foreman **never parses harness internals**. It reads the output stream (for logs, the "last line" and silence detection) and talks to the agent through the team tools and stdin.
- Harness integration contract (per [Harness adapters](./harness-adapters.md)):
  1. Foreman can start it with a working directory, environment and an initial prompt or system prompt file.
  2. It can call the team tools (an MCP server, configured per harness).
  3. It accepts stdin text while running.
- Per-harness adapter config covers: command, args, how to pass the system prompt, how to register the team-tools MCP server, and how to detect idle and "waiting for input" states.

## Concurrency

- Workspace setting: max running sessions (default 12, range 1–64). Instances beyond the limit are `queued` and start as slots free up. See [Agent runtime](./agent-runtime.md#concurrency).
- Requesters waiting on a reply don't hold a slot when the harness supports pause or resume.

## Non-functional requirements

- **Local first**: binds to localhost by default. No telemetry. Works offline, except harnesses and forges that need the network.
- **Security**: no user accounts in v1, but every request needs the per-install access token ([Security](./security.md#access-token)); binding to the network shows a warning. Secrets are never written to project folders or logs. Agents never receive forge tokens. See [Security](./security.md) for what this does and doesn't protect against.
- **Isolation**: separate clones and containers per instance; Team tools return only designated outputs; wiki write access limited to the scrum master or lead.
- **Reliability**: instances survive UI disconnects. On server restart, deployments are restored from `state.db`; running containers are re-attached, stopped ones restarted with the last task note ([Agent runtime](./agent-runtime.md#recovery)). PTY logs allow scrollback replay.
- **Performance**: 20+ concurrent sessions on a laptop. Terminal latency under 50 ms locally. File changes reflected in the UI within 1 s.
- **Observability**: per-instance logs, a team-message log, a request history, and a server log with harness exit codes.
- **Portability**: macOS and Linux first. Windows via WSL. On macOS Podman runs inside a VM (`podman machine`), see [Session runner](./session-runner.md).
