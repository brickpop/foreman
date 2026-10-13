---
type: spec
status: planned
tags: [model, sessions, agents, runtime]
---

# Instance

One [agent soul](./agent-soul.md) running in one [deployment](./deployment.md): a live harness process attached to a terminal, in its own container and sandbox ([Session runner](../architecture/session-runner.md)). Also called a session. Identified as `agent@project` (e.g. `mara@ledger`). Its terminal is live in [Sessions](../features/sessions.md); its lifecycle is in [Agent runtime](../architecture/agent-runtime.md).

## Schema

Table `instances`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | `agent@project`, unique | Can't collide: agent ids are unique across the workspace ([Team definition](./team-definition.md#schema)) |
| `deployment_id` | String → Deployment | yes | | | |
| `agent_id` | String | yes | | an agent of the deployment's team | |
| `status` | String | yes | `starting` | enum below | |
| `task` | String | | | | Current task note (e.g. "Waiting for a card"). Used to restart after a server restart |
| `pid` | Number | | | | Harness process id (inside the container, or the container's) |
| `pty_id` | String | | | | Handle of the attached terminal in the session runner (e.g. container id) |
| `sandbox_path` | String | yes | | | `<project>/.sandboxes/<agent>/` |
| `harness` | String | yes | the soul's harness | enum as in [Agent soul](./agent-soul.md) | Harness actually running: differs from the soul after a "Switch harness", an override that lasts until the instance is next restarted from the team file ([Agent soul](./agent-soul.md#harness-override)) |
| `harness_session_id` | String | | | | The harness's own session id, for native resume on restart ([Agent runtime](../architecture/agent-runtime.md#restarts)) |
| `started_at` | Timestamp | | | | |
| `last_output_at` | Timestamp | | | | Drives the "stuck" blocked request (no output for N min, default 20) |
| `restarts` | Number | yes | `0` | ≥ 0 | |

## Statuses

| Status | Meaning | Color ([Design system](../design/design-system.md)) |
|---|---|---|
| `queued` | Waiting for a free slot (max running sessions reached) | grey dashed outline (`--tx4`) |
| `starting` | Container, sandbox and harness being set up | violet (`--team`) |
| `running` | Working | green (`--ok`) |
| `idle` | Waiting at its prompt with nothing assigned (and not right after a handoff) | grey (`--tx4`) |
| `awaiting` | Waits on an open blocking approval from the CEO ([Request](./request.md#invariants)) | amber (`--acc`) |
| `done` | Just handed off, nothing assigned yet. Lasts until the next input or card assignment, then becomes `idle` (or `running`) | blue (`--info`) |
| `stopped` | Exited, crashed, rate-limited, or its deployment was stopped | red (`--err`) |

## Logs

The raw PTY byte stream is appended to `~/agents-ws/.foreman/logs/<agent>@<project>/<session-id>.log`, one file per session (a restart starts a new session id). It serves scrollback replay in the browser terminal. Logs rotate at a size cap ([Persistence](../architecture/persistence.md#session-logs)).

## Invariants

- One container and one sandbox per instance; instances never share a working tree.
- An instance in `awaiting` has at least one open blocking request.
- A crash, exit or rate limit sets `stopped` and raises a blocked [request](../features/requests.md).
- Instances survive UI disconnects.
