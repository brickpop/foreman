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
| `path` | String | yes | | unique; project-qualified (`<project>/wiki/<entry>`) | |
| `last_seen_version` | String | yes | | | The entry version the CEO last opened (the same version used to guard wiki writes) |

An entry shows a changed dot when its current version differs from `last_seen_version`, or it has no marker. Whatever the [Wiki reader](../features/wiki-reader.md) displays counts as seen, including the `index.md` shown when the Overview opens; the marker stores the version seen, so any later change re-lights the dot. When a project is added, every existing entry is marked as seen, so a new project doesn't start with every entry dotted.

Table `ui_prefs`: key/value preferences.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `key` | String | yes | | unique | e.g. `theme`, `open_panes`, `last_screen` |
| `value` | String (JSON) | yes | | | |

Known keys: `theme` (`system` default, `dark` or `light`), `open_panes` (the [Sessions](../features/sessions.md) panes, per deployment, restored when the deployment is opened again), `last_screen`.

Preferences live here, not in the browser, so they follow the CEO across browsers. The client caches `theme` in `localStorage` only to paint the right theme before the API answers (no flash on load); `ui_prefs` stays the source of truth.
