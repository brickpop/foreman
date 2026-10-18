---
type: spec
status: planned
tags: [model, config, workspace]
---

# Workspace settings

`~/agents-ws/.foreman/settings.toml`, in the [workspace repo](./workspace.md#the-workspace-repo): local, per-machine settings, not shared with anyone. Edited only through the [Workspace settings screen](../features/settings.md); each save is one commit. Forge tokens are not here: they are [Secrets](./secrets.md).

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `listen` | String | | `local` | enum: `local` (localhost), `lan` (local network) | `lan` shows a warning: the access token is the only protection ([Security](../architecture/security.md#access-token)) |
| `port` | Number | | `8888` | | The server listens on `localhost:8888` by default |
| `max_sessions` | Number | | `12` | 1–64 | Max running sessions at once. Instances beyond it are `queued` ([Agent runtime](../architecture/agent-runtime.md)) |
| `engine` | String | | `podman` | enum: `podman`, `direct` | The [execution engine](../architecture/building-blocks.md#1-execution-engine-runner) agents run on. `direct` runs them as plain processes on the host with **no isolation**; choosing it asks for confirmation and the Settings screen keeps a warning visible while it is on. Meant for tests and development. The engine is chosen when Foreman starts: a change takes effect on the next restart |
| `stuck_after_minutes` | Number | | `20` | > 0 | No output for this long raises a blocked [request](../features/requests.md) |
| `log_cap_mb` | Number | | `50` | > 0 | Session log size cap per instance before rotation ([Persistence](../architecture/persistence.md#session-logs)) |
| `project_profiles.<project>` | String | | `default` | a profile's name: lowercase letters, digits and dashes, at most 40 | The [profile](../architecture/session-runner.md#credentials) a project's agents run on (a harness and its login), set from its [Project settings](../features/project-settings.md); a project not named uses `default`, and choosing `default` removes its line. Here, not in `project.toml`: the lead can write the project repo, and profile names are this machine's (the CEO, 2026-10-18) |
| `forges.<kind>` | Map | | | `kind` ∈ `github`, `forgejo`, `gitea` | Host or owner per forge account; the token lives in [Secrets](./secrets.md) |
| `workspace_forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | The forge hosting the [workspace repo](./workspace.md#the-workspace-repo), where Sync pushes its `main`; `none`: it stays on this machine. Its account and token are the matching `forges.<kind>`; nothing is pushed automatically: the CEO pushes `main` with Sync ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)) |
| `workspace_forge.url` | String (URL) | when `kind` ≠ `none` | | | The workspace repo on the forge |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.merge_requested` | Boolean | | `true` | | "A merge is requested" |

There is no harness command setting (decided 2026-10-15): Foreman builds the agent image, so each harness runs its own command (`claude`, …); a `[harnesses]` table is reported as a field Foreman doesn't know. Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys.

## Invariants

- The workspace folder path itself is not a setting: it is where `foreman` was started, shown read-only.
- No secret is ever stored in this file.
