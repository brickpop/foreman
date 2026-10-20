---
type: spec
status: planned
tags: [architecture, runtime, sessions, team-tools, api]
---

# Architecture overview

Foreman is **one Go binary** that serves the API and the built SPA, supervises one containerised harness process per agent [instance](../models/instance.md), and routes everything that crosses between agents or reaches the CEO. Languages, libraries and repo layout are in [Tech stack](./tech-stack.md). This entry names the components and how they connect; each has its own entry.

It listens on `localhost:8888` by default ([Global settings](../models/global-settings.md) can switch it to the local network, see [Security](./security.md)).

## Components

The blocks Foreman is made of, their interfaces and how they connect are in [Building blocks](./building-blocks.md), the authoritative map. The rules below keep them swappable.

Three invariants hold across all of them:
- The UI never talks to a harness directly: terminal bytes flow through the Terminal block, which also tees them to the session log.
- Harnesses never talk to each other directly: only Messaging moves `output` between instances, and only within one [deployment](../models/deployment.md).
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
| Execution engine: where a harness process runs | `runner` | Podman (production), Direct (tests, dev, or an explicit no-isolation setting); tmux possible later ([Building blocks](./building-blocks.md#1-execution-engine-runner), [Session runner](./session-runner.md)) |
| Session driver: how Foreman talks to a harness | `harness` | PTY (v1); Agent Client Protocol later ([Harness adapters](./harness-adapters.md#session-drivers)) |
| Harness adapter: per-harness details | `harness` | Claude Code, Pi, Hermes Agent, custom; Codex later; the fake harness in tests |
| Session log: each session's raw history | `sessionlog` | files under `~/.foreman/logs/`; an in-memory log in tests |
| Screen: server-side terminal state | `terminal` (consumer) | `charmbracelet/x/vt`; a fake in the Terminal's tests ([Terminal state](./terminal-state.md)) |
| Terminal: one live process, its buffer and viewers | `terminal` | the Terminal block; a fake in the server's tests ([Building blocks](./building-blocks.md#2-terminal-terminal)) |
| Agent link: how agents reach Foreman | `agentlink` + `foreman-agent` | one loopback port, a token per instance ([Team tools → Transport](./team-tools.md#transport)) |
| Instance manager, Messaging, Workflow | `instances`, `messaging`, `workflow` | one each today ([Building blocks](./building-blocks.md)) |
| Image builder | `runner/<engine>` | libpod build API (Podman); a no-op (Direct) ([Agent images](./tech-stack.md#agent-images)) |
| Forge | `forge` | GitHub, Gitea (also Forgejo), none; a fake forge server in tests ([Forge adapters](./forge-adapters.md)) |
| Secret store | `secrets` | 0600 file (v1); OS keychain later ([Secrets](../models/secrets.md)) |
| Wiki engine | `wiki` | the agentic-wiki engine ([Wiki engine](./wiki-engine.md)) |
| Runtime state | `store` | SQLite (also in tests, in a temp dir) ([Persistence](./persistence.md)) |
| Access guard: who may reach the server | `access` | the per-install token guard ([Security](./security.md#access-token)); real user auth if ever needed |
| The home, as the server sees it: projects, teams, settings | `home` | the folder on disk and the projects' records |
| UI files | `ui` | embedded build, or a folder on disk (`--ui-dir`) |

### In the UI

The same idea holds in the browser, scaled to it:

1. **Screens never talk to the network.** All server access goes through typed client interfaces in `ui/src/api/` (one per area: home, sessions, requests…). The transport behind them (HTTP, SSE, WebSocket) can change without touching a component, and tests give components an in-memory client.
2. **Third-party widgets sit behind our own component.** The terminal is our `Terminal` component with our props, not xterm.js used directly in screens; the same goes for the markdown view, drag and drop, and the Radix primitives. Replacing a library is then a change in one file.
3. **Components get data through props or hooks, never through globals.** The composition root is `ui/src/main.tsx`, which picks the real clients and provides them to the tree. An area the server doesn't serve yet refuses every call with the code `NOT_BUILT` (`isNotBuilt`) without a request, and its screen shows the `NotBuilt` panel; it is never retried.
4. **Design tokens, not literal values.** Colours, spacing and type come from the tokens in [Design system](../design/design-system.md), so the look can change in one place.

## Process model

- **One container per running instance**, decided: every harness runs in a rootless Podman container with only its own worktrees (and their repos' `.git`, read-only: Foreman writes git for the agents) and the project repo mounted ([Session runner](./session-runner.md)). Inside it the harness is attached to a TTY.
- Foreman **never parses harness internals**. It reads the output stream (for logs and silence detection), learns what an agent is doing from its harness's hooks (status and [activity line](./harness-adapters.md#activity-lines)), and talks to the agent through the team tools and stdin.
- Harness integration contract (per [Harness adapters](./harness-adapters.md)):
  1. Foreman can start it with a working directory, environment and an initial prompt or system prompt file.
  2. It can call the team tools (an MCP server, configured per harness).
  3. It accepts stdin text while running.
- Per-harness adapter config covers: command, args, how to pass the system prompt, how to register the team-tools MCP server, and how to detect idle and "waiting for input" states.

## Concurrency

- Global setting: max running sessions (default 12, range 1–64). Instances beyond the limit are `queued` and start as slots free up. See [Agent runtime](./agent-runtime.md#concurrency).
- Requesters waiting on a reply don't hold a slot when the harness supports pause or resume.

## Non-functional requirements

- **Local first**: binds to localhost by default. No telemetry. Works offline, except harnesses and forges that need the network.
- **Security**: no user accounts in v1, but every request needs the per-install access token ([Security](./security.md#access-token)); binding to the network shows a warning. Secrets are never written into a repo or a log. Agents never receive forge tokens. See [Security](./security.md) for what this does and doesn't protect against.
- **Isolation**: a separate worktree and container per instance; Team tools return only designated outputs; knowledge-base write access limited to the lead.
- **Reliability**: instances survive UI disconnects. A clean exit puts every agent to sleep; on server restart, deployments are restored from `state.db`; containers still running (after a crash) are re-attached, stopped ones restarted with the last task note ([Agent runtime](./agent-runtime.md#recovery)). PTY logs allow scrollback replay.
- **Performance**: 20+ concurrent sessions on a laptop. Terminal latency under 50 ms locally. File changes reflected in the UI within 1 s.
- **Observability**: per-instance logs, a team-message log, a request history, and a server log with harness exit codes.
- **Portability**: macOS and Linux first. Windows via WSL. On macOS Podman runs inside a VM (`podman machine`), see [Session runner](./session-runner.md).
