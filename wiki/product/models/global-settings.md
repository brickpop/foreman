---
type: spec
status: planned
tags: [model, config, home]
---

# Global settings

`~/.foreman/config/settings.toml`, in the [global repo](./home.md#the-global-repo): per-machine settings, not shared with anyone. Edited through [Settings](../features/settings.md); each save is one commit.

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `port` | Number | | `8888` | 1–65535 | The web UI's port, on `127.0.0.1` only (no local-network mode; the CEO, 2026-10-30). The `--port` flag wins over it ([Starting Foreman](./home.md#starting-foreman)); the agent port is the next one. A change takes effect on the next restart. Settings has no row for it: it is edited in the file |
| `max_sessions` | Number | | `12` | 1–64 | Max running sessions at once. Instances beyond it are `queued` ([Agent runtime](../architecture/agent-runtime.md)) |
| `engine` | String | | `podman` | enum: `podman`, `direct` | The [execution engine](../architecture/building-blocks.md#1-execution-engine-runner) agents run on. `direct` runs them as plain processes on the host with **no isolation**; choosing it asks for confirmation and the Settings screen keeps a warning visible while it is on. Meant for tests and development. The engine is chosen when Foreman starts: a change takes effect on the next restart |
| `stuck_after_minutes` | Number | | `20` | > 0 | An agent mid-turn with no hook event for this long gets a blocked [request](../features/requests.md) ([Agent runtime → Stuck agents](../architecture/agent-runtime.md#stuck-agents)); read at each check |
| `log_cap_mb` | Number | | `50` | > 0 | The most each agent's session logs hold, oldest first out ([Persistence → Session logs](../architecture/persistence.md#session-logs)); read as logs are written |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.merge_requested` | Boolean | | `true` | | "A merge is requested" |
| `instructions` | String (Markdown) | | empty | | The CEO's standing instructions, added at the end of every team agent's [session prompt](../architecture/session-prompt.md#the-ceos-instructions) at its next start; set in [Settings](../features/settings.md#instructions-for-every-agent). Foreman writes it as a multi-line string (`"""`), so it reads as typed |

| `projects_folder` | String (path) | | `~/workspace/foreman` | `~`, `~/…` or an absolute path | Where New project suggests a project's [workspace](./project-config.md#the-workspace), `<projects folder>/<name>`; created with the first project made in it. Kept as a `~` path, so it carries to another machine. Set in [Settings → System](../features/settings.md#projects) |
There is no harness command setting (decided 2026-10-15): Foreman builds the agent image, so each harness runs its own command (`claude`, …); a `[harnesses]` table is reported as a field Foreman doesn't know. Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys; there is no `listen` (the CEO, 2026-10-30: Foreman stays on this machine).

## Invariants

- The home's path is not a setting: it is `~/.foreman` (or `--home`), shown read-only.
- A project's profile is not here: it is on the project's record ([Project → its record](./project-config.md#the-project-record)), set from its [Project settings](../features/project-settings.md) (the CEO, 2026-10-20).
- No secret is ever stored in this file.
