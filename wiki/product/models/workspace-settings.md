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
| `harnesses.<harness>.command` | String | | `claude`, `codex`, `pi`, `hermes` | | Command per harness, run inside the harness image ([Agent images](../architecture/tech-stack.md#agent-images)); the UI shows detection status ("found · version" or "not set") ([Harness adapters](../architecture/harness-adapters.md)) |
| `stuck_after_minutes` | Number | | `20` | > 0 | No output for this long raises a blocked [request](../features/requests.md) |
| `team_message_markers` | Boolean | | `true` | | Show the marker strip beside session terminals, toggled on the [Settings screen](../features/settings.md) as "Show agent messages beside terminals". Markers are never written into a terminal ([Sessions](../features/sessions.md#markers-beside-the-terminal)) |
| `log_cap_mb` | Number | | `50` | > 0 | Session log size cap per instance before rotation ([Persistence](../architecture/persistence.md#session-logs)) |
| `forges.<kind>` | Map | | | `kind` ∈ `github`, `forgejo`, `gitea` | Host or owner per forge account; the token lives in [Secrets](./secrets.md) |
| `workspace_forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | The forge hosting the [workspace repo](./workspace.md#the-workspace-repo), where Create PR on the Chief of Staff's team-change merge requests opens the PR; `none`: Merge to main only ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)). Its account and token are the matching `forges.<kind>`; nothing is pushed automatically: the CEO pushes `main` with Sync ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)) |
| `workspace_forge.url` | String (URL) | when `kind` ≠ `none` | | | The workspace repo on the forge |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.merge_requested` | Boolean | | `true` | | "A merge is requested" |

Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys.

## Invariants

- The workspace folder path itself is not a setting: it is where `foreman` was started, shown read-only.
- No secret is ever stored in this file.
