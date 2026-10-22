---
type: spec
status: planned
tags: [model, config, home]
---

# Global settings

`~/.foreman/config/settings.toml`, in the [global repo](./home.md#the-global-repo): per-machine settings, not shared with anyone. Edited through [Settings](../features/settings.md); each save is one commit. Forge tokens are not here: they are [Secrets](./secrets.md).

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `listen` | String | | `local` | enum: `local` (localhost), `lan` (local network) | `lan` shows a warning: the access token is the only protection ([Security](../architecture/security.md#access-token)) |
| `port` | Number | | `8888` | | The server listens on `localhost:8888` by default |
| `max_sessions` | Number | | `12` | 1–64 | Max running sessions at once. Instances beyond it are `queued` ([Agent runtime](../architecture/agent-runtime.md)) |
| `engine` | String | | `podman` | enum: `podman`, `direct` | The [execution engine](../architecture/building-blocks.md#1-execution-engine-runner) agents run on. `direct` runs them as plain processes on the host with **no isolation**; choosing it asks for confirmation and the Settings screen keeps a warning visible while it is on. Meant for tests and development. The engine is chosen when Foreman starts: a change takes effect on the next restart |
| `stuck_after_minutes` | Number | | `20` | > 0 | An agent mid-turn with no hook event for this long gets a blocked [request](../features/requests.md) ([Agent runtime → Stuck agents](../architecture/agent-runtime.md#stuck-agents)); read at each check |
| `sleep_after_minutes` | Number | | `0` | ≥ 0 | An agent idle this long, with nothing else running in its session, goes to sleep and frees its slot ([Agent runtime → Idle agents sleep](../architecture/agent-runtime.md#idle-agents-sleep)); `0`: never; read at each check |
| `log_cap_mb` | Number | | `50` | > 0 | The most each agent's session logs hold, oldest first out ([Persistence → Session logs](../architecture/persistence.md#session-logs)); read as logs are written |
| `forges.<kind>` | Map | | | `kind` ∈ `github`, `forgejo`, `gitea` | Host or owner per forge account; the token lives in [Secrets](./secrets.md) |
| `global_forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | The forge hosting the [global repo](./home.md#the-global-repo), where Sync pushes its `main`; `none`: it stays on this machine. Its account and token are the matching `forges.<kind>`; nothing is pushed automatically: the CEO pushes `main` with Sync ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)) |
| `global_forge.url` | String (URL) | when `kind` ≠ `none` | | | The global repo on the forge |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.merge_requested` | Boolean | | `true` | | "A merge is requested" |

There is no harness command setting (decided 2026-10-15): Foreman builds the agent image, so each harness runs its own command (`claude`, …); a `[harnesses]` table is reported as a field Foreman doesn't know. Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys.

## Invariants

- The home's path is not a setting: it is `~/.foreman` (or `--home`), shown read-only.
- A project's profile is not here: it is on the project's record ([Project → its record](./project-config.md#the-project-record)), set from its [Project settings](../features/project-settings.md) (the CEO, 2026-10-20).
- No secret is ever stored in this file.
