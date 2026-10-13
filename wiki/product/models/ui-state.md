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

An entry shows a changed dot when its current version differs from `last_seen_version`, or it has no marker.

Table `ui_prefs`: key/value preferences.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `key` | String | yes | | unique | e.g. `theme`, `open_panes`, `last_screen` |
| `value` | String (JSON) | yes | | | |

Known keys: `theme` (`dark` default, or `light`), `open_panes` (Sessions panes per deployment), `last_screen`.

## Open questions

1. **Theme persistence.** The PRD stores it in `ui_prefs`; the prototype uses browser `localStorage` (`foreman-theme`). Recommendation: `ui_prefs` via the API, so it follows the CEO across browsers; the client caches it locally to avoid a flash of the wrong theme on load. (Whether a `System` option exists is a [Settings screen](../features/settings.md#open-questions) question.)
2. **What "changed since you last looked" means.** Does the entry shown by default (`index.md`) count as seen when the Overview opens (the prototype says no)? Is it tracked per file version, so a change after viewing re-lights the dot? Recommendation: whatever is displayed counts as seen (including the initial `index.md`), and the marker stores the version seen, so any later change re-lights it ([Wiki reader](../features/wiki-reader.md)).
3. **Never-seen entries.** On first use every entry has no marker, so everything would show a dot. Recommendation: when a project is added, mark all existing entries as seen.
4. **Pane persistence scope.** Should the open panes (`open_panes`) be restored, and per deployment? Recommendation: yes, per deployment ([Sessions](../features/sessions.md)).
