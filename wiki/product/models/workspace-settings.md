---
type: spec
status: planned
tags: [model, config, workspace]
---

# Workspace settings

`~/agents-ws/.foreman/settings.yaml`: local, per-machine settings, not shared with anyone. Edited only through the [Workspace settings screen](../features/settings.md). Forge tokens are not here: they are [Secrets](./secrets.md).

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `listen` | String | | `local` | enum: `local` (localhost), `lan` (local network) | `lan` shows a warning: the access token is the only protection ([Security](../architecture/security.md#access-token)) |
| `port` | Number | | `8888` | | The server listens on `localhost:8888` by default |
| `max_sessions` | Number | | `12` | 1–64 | Max running sessions at once. Instances beyond it are `queued` ([Agent runtime](../architecture/agent-runtime.md)) |
| `harnesses.<harness>.command` | String | | `claude`, `codex`, `pi`, `hermes` | | Command per harness, run inside the harness image ([Agent images](../architecture/tech-stack.md#agent-images)); the UI shows detection status ("found · version" or "not set") ([Harness adapters](../architecture/harness-adapters.md)) |
| `stuck_after_minutes` | Number | | `20` | > 0 | No output for this long raises a blocked [request](../features/requests.md) |
| `team_message_markers` | Boolean | | `true` | | Show team-message marker lines (⇄) in terminals, toggled on the [Settings screen](../features/settings.md) as "Show agent messages in terminals". It hides the lines Foreman injects into the view, never the harness's own output ([Sessions](../features/sessions.md)) |
| `log_cap_mb` | Number | | `50` | > 0 | Session log size cap per instance before rotation ([Persistence](../architecture/persistence.md#session-logs)) |
| `forges.<kind>` | Map | | | `kind` ∈ `github`, `forgejo`, `gitea` | Host or owner per forge account; the token lives in [Secrets](./secrets.md) |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.pr_ready` | Boolean | | `true` | | "A PR is ready to merge" |

Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys.

## Invariants

- The workspace folder path itself is not a setting: it is where `foreman` was started, shown read-only.
- No secret is ever stored in this file.
