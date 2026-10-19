---
type: spec
status: planned
tags: [model, sessions, agents, runtime]
---

# Instance

One [agent soul](./agent-soul.md) running in one [deployment](./deployment.md): a live harness process attached to a terminal, in its own container and worktrees ([Session runner](../architecture/session-runner.md)). Also called a session. Identified as `agent@project` (e.g. `mara@ledger`). Its terminal is live in [Sessions](../features/sessions.md); its lifecycle is in [Agent runtime](../architecture/agent-runtime.md).

## Schema

Table `instances`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | `agent@project`, unique | Can't collide: agent ids are unique across the workspace ([Team definition](./team-definition.md#schema)) |
| `deployment_id` | String → Deployment | yes | | | |
| `agent_id` | String | yes | | an agent of the deployment's team | |
| `status` | String | yes | `starting` | enum below | Set at creation: `asleep`, or `queued` for the lead on deploy |
| `task` | String | | | | Current task note (e.g. "Waiting for a card"). Used to restart after a server restart |
| `pid` | Number | | | | Harness process id (inside the container, or the container's) |
| `pty_id` | String | | | | Handle of the attached terminal in the session runner (e.g. container id) |
| `sandbox_path` | String | yes | | | `<project>/.sandboxes/<agent>/`: its worktrees, one per code repo |
| `harness` | String | yes | its project's profile's harness | a supported harness | The harness it runs on: its project's [profile](../architecture/session-runner.md#credentials)'s, set at deploy and again at each start, so a change of the project's profile reaches it at its next start |
| `profile` | String | yes | its project's profile | a profile name, or empty | The [profile](../architecture/session-runner.md#credentials) its current run started on, set at deploy and at each start; empty for a run from before it was kept (2026-10-18). Project settings lists running agents whose profile isn't the project's ([Project settings](../features/project-settings.md)) |
| `harness_session_id` | String | | | | The harness's own session id, for native resume on restart ([Agent runtime](../architecture/agent-runtime.md#restarts)) |
| `started_at` | Timestamp | | | | |
| `last_output_at` | Timestamp | | | | Drives the "stuck" blocked request (no output for N min, default 20) |
| `restarts` | Number | yes | `0` | ≥ 0 | |

Not stored: its **activity line** (`doing`: a line and its time), what its harness last reported doing in the current run, kept in memory by the Instance manager and sent with the instances ([Harness adapters → Activity lines](../architecture/harness-adapters.md#activity-lines)). **Phasing:** `task` and `last_output_at` are never set yet, so neither is shown; the agent Sessions opens a deployment on is the one with the latest activity line.

## Statuses

| Status | Meaning | Color ([Design system](../design/design-system.md)) |
|---|---|---|
| `asleep` | No container: a member with no work yet (only the lead starts on deploy). Woken by the CEO's Start, or by a delivery to it ([Agent runtime → Lazy starts](../architecture/agent-runtime.md#lazy-starts)) | grey solid outline (`--tx4`) |
| `queued` | Waiting for a free slot (max running sessions reached) | grey dashed outline (`--tx4`) |
| `starting` | Container, worktrees and harness being set up | violet (`--team`) |
| `running` | Working | green (`--ok`) |
| `idle` | Waiting at its prompt with nothing assigned (and not right after a handoff) | grey (`--tx4`) |
| `awaiting` | Waits on the CEO: an open `escalate(approval)`, or the harness reported it is waiting for the user ([Request](./request.md#invariants), [Protocol](../architecture/protocol.md#waiting-for-the-ceo)) | amber (`--acc`) |
| `done` | Just handed off, nothing assigned yet. Lasts until the next input or card assignment, then becomes `idle` (or `running`) | blue (`--info`) |
| `stopped` | Exited, crashed, rate-limited, or its deployment was stopped | red (`--err`) |

## Logs

The raw terminal byte stream is appended to `~/agents-ws/.foreman/logs/<agent>@<project>/<session-id>.log`, one file per session (a restart starts a new session id). It is the session's history and audit trail; the browser terminal is restored from a snapshot of the server-side screen, not from the log ([Terminal state](../architecture/terminal-state.md)). Logs rotate at a size cap ([Persistence](../architecture/persistence.md#session-logs)).

## Invariants

- One container and one worktree per code repo per instance; instances never share a working tree (they share each repo's `.git`, read-only to them).
- An instance in `awaiting` has at least one open blocking request.
- A crash, exit or rate limit sets `stopped` and raises a blocked [request](../features/requests.md).
- Instances survive UI disconnects.
