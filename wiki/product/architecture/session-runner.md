---
type: spec
status: planned
tags: [architecture, runtime, sessions, security]
---

# Session runner

Where and how a harness process actually runs. The session supervisor ([Agent runtime](./agent-runtime.md)) never starts processes itself: it goes through a small **Runner** interface, so the execution backend is swappable without touching anything else.

**Decided:** every agent instance runs in a **rootless Podman container from day one**. A local-PTY runner exists for hermetic tests and development.

## The Runner interface

Conceptually (exact Go signatures are an implementation detail):

| Operation | Does |
|---|---|
| `Start(spec) → handle` | Starts a session from a spec: `{name, image, mounts, env, cmd, workdir, tty size, labels}`. `labels` carry the instance id so sessions can be found again |
| `Attach(handle) → stream` | Bidirectional byte stream to the session's TTY (stdin in, merged stdout/stderr out). Several attaches over time; the supervisor holds one |
| `Resize(handle, cols, rows)` | Propagates a terminal resize to the TTY |
| `Stop(handle, grace)` | Graceful stop (SIGTERM, then kill after `grace`) |
| `Wait(handle) → exit status` | Blocks until the process exits; returns the exit code (used to raise blocked requests) |
| `List() → []handle` | Running sessions created by Foreman (by label), for [recovery](./agent-runtime.md#recovery) after a Foreman restart |

The supervisor tees every byte read from `Attach` into the instance's session log ([Persistence](./persistence.md)) before fanning it out to WebSocket clients ([API](./api.md#pty-websocket)).

## Podman runner (production)

- Each instance is a container named from its instance id, labelled `foreman.instance=<id>`, `foreman.deployment=<id>`.
- **Mounts, and nothing else of the host:**
  - the instance's sandbox clone `<project>/.sandboxes/<agent>/` → read-write, as the working directory
  - the project `wiki/` → read-write only for the scrum master (or the lead when the team has no scrum master), read-only for everyone else ([Sandboxes and git policy](./sandboxes-and-git.md))
  - no `~/.config`, no `~/.ssh`, no keychain, no `.foreman/` (secrets, state, team files), no other sandboxes, no project repos.
- **Harness credentials** are passed in, never mounted from the host config: e.g. a Claude Code long-lived OAuth token from `claude setup-token` via env (on macOS Claude Code keeps its credentials in the Keychain, which can't be mounted anyway). The token itself is a Foreman [secret](../models/secrets.md). Per-harness auth is an open question.
- **TTY:** the harness is the container's main process with a TTY, so the container keeps running when Foreman exits. After a Foreman restart, `List` finds it by label and Foreman re-attaches.
- **User mapping:** `--userns=keep-id`, so files written into the sandbox and wiki are owned by the CEO's user, not a subordinate UID.
- **Network:** containers need the network for model APIs. They reach Foreman's ACP listener at `host.containers.internal` ([ACP](./acp.md#transport)).
- **macOS:** Podman runs inside a VM (`podman machine`). Bind mounts cross the VM boundary (virtiofs), which is slower for heavy file I/O (e.g. `npm install`) and needs the workspace path shared with the machine. Unix sockets don't cross that boundary.
- **Images:** containers need both the harness and the project's toolchain to run `checks` (`npm test`, `cargo test`, …). See open questions.

## Local PTY runner (tests and development)

- Spawns the command directly on the host in a pseudo-terminal (the approach of the original PTY proof of concept).
- Used with a **fake scripted harness** (a small program that prints scripted output, calls ACP tools and reacts to stdin) so the supervisor, ACP router, request engine and pipeline can be tested end-to-end, fast and without Podman or a model.
- Gives no isolation. It must never be selectable for real agents without an explicit, visible opt-in (see open questions).
- Sessions die with Foreman, so `List` returns nothing after a restart.

A tmux-based runner (sessions survive restarts, attachable from a real terminal) could be a third implementation later. Not planned.

Images are built by Foreman in layers; see [Tech stack](./tech-stack.md#agent-images).

## Open questions

1. **Podman API vs CLI.** Talk to Podman through the libpod REST API over its socket with plain HTTP (recommended: one dependency-free client, structured errors, attach as a hijacked stream), or shell out to the `podman` CLI (`podman run -dt`, `podman attach` inside a PTY)? The official Go bindings module is avoided either way: it pulls in a very large dependency tree.
2. **Per-harness credentials and their scope.** How each harness authenticates inside a container: Claude Code (OAuth token via `CLAUDE_CODE_OAUTH_TOKEN`, or API key), Codex (API key or ChatGPT login file?), Pi and Hermes (to be confirmed). Are they secrets Foreman stores (as `harness/<harness>` in [Secrets](../models/secrets.md)) and injects as container environment, or does the CEO mount the harness's own config? Is one credential shared by all instances, or one per instance? These are model-provider credentials, not forge tokens, so "agents never receive forge tokens" still holds, but they do reach the agent ([Security](./security.md)). Recommendation: store and inject per container; spike per harness. Whether provider terms allow it is in [Harness adapters](./harness-adapters.md#open-questions).
3. **Scrollback across restarts.** While Foreman is down, output is not logged. Can the Podman runner replay it (container logs with a TTY), or is that gap accepted? Recommendation: accept the gap in v1 and print a `ⓘ Foreman restarted, output may be missing` system line.
4. **PTY runner in production.** Should the PTY runner be available for real use (e.g. on machines without Podman) behind a loud "no isolation" setting, or test-only? Recommendation: test and dev only in v1.
5. **Resource limits.** CPU/memory limits per container, to keep 20+ sessions usable on a laptop. Recommendation: none by default, a workspace setting later.
