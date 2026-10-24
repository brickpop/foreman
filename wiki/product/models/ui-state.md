---
type: spec
status: planned
tags: [model, ui, wiki, runtime]
---

# UI state

Small pieces of CEO-facing state that Foreman keeps in `state.db` so they survive reloads: read markers for the wiki, and UI preferences.

## Schema

Table `read_markers`: what the CEO has seen in each wiki, for the "changed since you last looked" dots in the [Wiki reader](../features/wiki-reader.md).

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `path` | String | yes | | unique; project-qualified (`<project>/<entry>`, the entry's path in the project's wiki) | |
| `last_seen_version` | String | yes | | | The entry version the CEO last opened (the same version used to guard wiki writes) |

An entry shows a changed dot when its current version differs from `last_seen_version`, or it has no marker. Whatever the [Wiki reader](../features/wiki-reader.md) displays counts as seen, including the `index.md` shown when the Overview opens; the marker stores the version seen, so any later change re-lights the dot. When a project is added, every existing entry is marked as seen, so a new project doesn't start with every entry dotted.

Table `ui_prefs`: key/value preferences.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `key` | String | yes | | unique | e.g. `sessions_view`, `last_screen` |
| `value` | String (JSON) | yes | | | |

Known keys: `sessions_view` (each deployment's last agent and Activity in [Sessions](../features/sessions.md#the-url), so its sidebar link comes back to it; kept only while the app runs until this table exists), `last_screen`.

Preferences live here, not in the browser, so they follow the CEO across browsers. They sit in `state.db` beside the deployments they name. The **theme** is not here (the CEO, 2026-10-18): it names nothing in the state, so it is one of the CEO's preferences, `theme` in `~/.foreman/preferences.toml` (`system` default, `dark` or `light`; `GET/PUT /api/preferences`, [Settings → Appearance](../features/settings.md#appearance)). The browser keeps a copy only to paint the first frame before the server answers (no flash on load); the file stays the source of truth. Built 2026-10-18.
