---
type: spec
status: planned
tags: [architecture, runtime, sessions, acp, api]
---

# Architecture overview

Foreman is **one Go binary** that serves the API and the built SPA, supervises one containerised harness process per agent [instance](../models/instance.md), and routes everything that crosses between agents or reaches the CEO. Languages, libraries and repo layout are in [Tech stack](./tech-stack.md). This entry names the components and how they connect; each has its own entry.

It listens on `localhost:7420` by default ([Workspace settings](../models/workspace-settings.md) can switch it to the local network, see [Security](./security.md)).

## Components

| # | Component | Job | Entry |
|---|---|---|---|
| 1 | **HTTP server + SPA** | Serves the JSON API, SSE, the PTY WebSockets and the embedded UI build | [API](./api.md) |
| 2 | **Workspace indexer** | Scans the [workspace](../models/workspace.md) for projects and repos. Watches config, team files and wikis; pushes changes to clients within ~1 s | [Persistence](./persistence.md) |
| 3 | **Session supervisor** | Starts harnesses through the [session runner](./session-runner.md), one per instance. Owns lifecycle, restarts, the concurrency queue and logs. Exposes PTY I/O over WebSocket | [Agent runtime](./agent-runtime.md) |
| 4 | **Sandbox manager** | Creates and refreshes per-instance clones, checks out branches, enforces branch policy | [Sandboxes and git policy](./sandboxes-and-git.md) |
| 5 | **ACP router** | Exposes the agent tools to harnesses (MCP), routes messages between instances of one deployment, stores them, injects replies into the requester's session | [ACP](./acp.md) |
| 6 | **Request engine** | Creates, deduplicates and resolves [requests](../features/requests.md) (agent-raised and Foreman-raised). Resolving one writes the answer back to the agent and resumes it | [Request](../models/request.md) |
| 7 | **Pipeline tracker** | Tracks handoffs and stages per work item; drives forge adapters | [Pipeline and PRs](../features/pipeline-and-prs.md), [Pipeline item](../models/pipeline-item.md) |
| 8 | **Forge adapters** | GitHub, Forgejo, Gitea (REST) and `none` (local). Push lead branches, open PRs, read checks, merge, comment on send-back | [Forge adapters](./forge-adapters.md) |
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
                                            │                       MCP (ACP listener, token)   │
                                        ACP router ◀────────────────────────────────────────────┘
                                            │
                              sandbox manager · forge adapters · state.db
```

- The UI never talks to a harness directly: PTY bytes flow through the supervisor, which also tees them to the session log.
- Harnesses never talk to each other directly: only the ACP router moves `output` between instances, and only within one [deployment](../models/deployment.md).
- Harnesses never touch a forge, a secret or another sandbox: pushes to forges are done by Foreman ([Sandboxes and git policy](./sandboxes-and-git.md)).

## Process model

- **One container per running instance**, decided: every harness runs in a rootless Podman container with only its own sandbox and the wiki mounted ([Session runner](./session-runner.md)). Inside it the harness is attached to a TTY.
- Foreman **never parses harness internals**. It reads the output stream (for logs, the "last line" and silence detection) and talks to the agent through the ACP tools and stdin.
- Harness integration contract (per [Harness adapters](./harness-adapters.md)):
  1. Foreman can start it with a working directory, environment and an initial prompt or system prompt file.
  2. It can call the ACP tools (an MCP server, configured per harness).
  3. It accepts stdin text while running.
- Per-harness adapter config covers: command, args, how to pass the system prompt, how to register the ACP/MCP server, and how to detect idle and "waiting for input" states.

## Concurrency

- Workspace setting: max running sessions (default 12, range 1–64). Instances beyond the limit are `queued` and start as slots free up. See [Agent runtime](./agent-runtime.md#concurrency).
- Requesters waiting on a reply don't hold a slot when the harness supports pause or resume.

## Non-functional requirements

- **Local first**: binds to localhost by default. No telemetry. Works offline, except harnesses and forges that need the network.
- **Security**: no user accounts in v1, but every request needs the per-install access token ([Security](./security.md#access-token-decided)); binding to the network shows a warning. Secrets are never written to project folders or logs. Agents never receive forge tokens. See [Security](./security.md) for what this does and doesn't protect against.
- **Isolation**: separate clones and containers per instance; ACP returns only designated outputs; wiki write access limited to the scrum master or lead.
- **Reliability**: instances survive UI disconnects. On server restart, deployments are restored from `state.db`; running containers are re-attached, stopped ones restarted with the last task note ([Agent runtime](./agent-runtime.md#recovery)). PTY logs allow scrollback replay.
- **Performance**: 20+ concurrent sessions on a laptop. Terminal latency under 50 ms locally. File changes reflected in the UI within 1 s.
- **Observability**: per-instance logs, an ACP log, a request history, and a server log with harness exit codes.
- **Portability**: macOS and Linux first. Windows via WSL. On macOS Podman runs inside a VM (`podman machine`), see [Session runner](./session-runner.md).
