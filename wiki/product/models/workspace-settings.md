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
| `listen` | String | | `local` | enum: `local` (localhost), `lan` (local network) | `lan` shows a warning: the access token is the only protection ([Security](../architecture/security.md#access-token-decided)) |
| `port` | Number | | `7420` | | The server listens on `localhost:7420` by default |
| `max_sessions` | Number | | `12` | 1–64 | Max running sessions at once. Instances beyond it are `queued` ([Agent runtime](../architecture/agent-runtime.md)) |
| `harnesses.<harness>.command` | String | | `claude`, `codex`, `pi`, `hermes` | | Command per harness, resolved on PATH; the UI shows detection status ("found · version" or "not set") ([Harness adapters](../architecture/harness-adapters.md)) |
| `stuck_after_minutes` | Number | | `20` | > 0 | No output for this long raises a blocked [request](../features/requests.md) |
| `acp_markers` | Boolean | | `true` | | Show ACP marker lines (⇄) in terminals ([Sessions](../features/sessions.md)) |
| `forges.<kind>` | Map | | | `kind` ∈ `github`, `forgejo`, `gitea` | Host or owner per forge account; the token lives in [Secrets](./secrets.md) |
| `notifications.needs_me` | Boolean | | `true` | | "An agent needs me" ([Notifications](../features/notifications.md)) |
| `notifications.finishes` | Boolean | | `true` | | "An agent finishes" |
| `notifications.pr_ready` | Boolean | | `true` | | "A PR is ready to merge" |

Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys.

## Invariants

- The workspace folder path itself is not a setting: it is where `foreman` was started, shown read-only.
- No secret is ever stored in this file.

## Open questions

1. **Where is the ACP-marker toggle?** The PRD calls it a workspace setting (§10.3), but the Settings screen (§10.9) doesn't list it and the prototype only has a hidden component prop. Recommendation: a setting here, shown on the [Settings screen](../features/settings.md) as "Show agent messages in terminals". Since markers are written into the PTY stream, hiding them applies to the lines Foreman injects into the view, not to the harness's own output ([Sessions](../features/sessions.md)).
