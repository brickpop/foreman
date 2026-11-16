---
type: spec
status: planned
tags: [model, config, home]
---

# Global settings

`~/.foreman/settings.toml`, a plain file in Foreman's [home](./home.md), in no repo: this machine's settings, not shared with anyone (the CEO, 2026-11-08). It can't live in [Foreman's folder](./home.md#foremans-folder), since it names it. Edited through [Settings](../features/settings.md) (`GET`/`PUT /api/settings`); a save rewrites the file and commits nothing. The CEO's preferences (the look) are settings too: there is no separate preferences file (the CEO, 2026-11-08).

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
| `theme` | String | | `system` | enum: `system`, `dark`, `light` | The UI's theme ([Settings → Appearance](../features/settings.md#appearance)). The browser keeps a copy only to paint the first frame before the server answers; the file stays the source of truth |
| `accent` | String | | `blue` | enum: `blue`, `green`, `purple`, `teal`, `orange`, `pink`, `amber` | The UI's accent color; first-frame copy as for `theme` |
| `terminal_font` | String | | `jetbrains-mono` | enum: `jetbrains-mono`, `fira-code`, `source-code-pro`, `system` | The terminal's font; `system` is the operating system's monospace |
| `terminal_font_size` | Number | | `13` | 10–24 | The terminal's font size, in px |
| `foreman_folder` | String (path) | | `~/foreman` | `~`, `~/…` or an absolute path; after `~`, inside `$HOME`, neither inside Foreman's home nor around it | [Foreman's folder](./home.md#foremans-folder): Foreman's HQ, the projects, the agents' worktrees. Kept as a `~` path. Read at start only: a change takes effect at the next start, and one Foreman can't use stops the start, saying why. No UI changes it (the CEO, 2026-11-15): Settings → System shows it read-only; the CEO moves the folder by hand with Foreman stopped and edits the file |

An invalid `theme`, `accent`, `terminal_font` or `terminal_font_size` reads as its default, the problem still reported, so the UI always gets a look it has.

The CEO's standing instructions to every agent are not a setting: they are `instructions.md` in [Foreman's HQ](./home.md#foremans-hq), the one part of the setup that travels between machines (the CEO, 2026-11-15).

There is no harness command setting (decided 2026-10-15): Foreman builds the agent image, so each harness runs its own command (`claude`, …); a `[harnesses]` table is reported as a field Foreman doesn't know. Field names are a proposal; the PRD names the contents ("listen, limits, harness commands, notifications"), not the keys; there is no `listen` (the CEO, 2026-10-30: Foreman stays on this machine).

## Invariants

- The home's path is not a setting: it is `~/.foreman` (or `--home`), shown read-only.
- A project's profile is not here: it is on the project's record ([Project → its record](./project-config.md#the-project-record)), set from its [Project settings](../features/project-settings.md) (the CEO, 2026-10-20).
- No secret is ever stored in this file.
