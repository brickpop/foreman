---
type: task
status: todo
priority: medium
tags: [feature]
epic: projects-anywhere
---

# Settings by topic

**Why:** Settings has two tabs, "Workspace" (`settings.toml`) and "Global" (profiles, maintenance, appearance). With no workspace any more both are machine-wide, so that split no longer says anything ([Settings](../product/features/settings.md)).

**Decided (the CEO, 2026-10-20):** tabs by topic, each row tagged with where it is kept (*global repo* for `config/settings.toml`, *this machine* for the rest):

- **Agents:** profiles, engine, max sessions, stuck timeout, log cap.
- **Git & forges:** forge accounts, the global repo's forge (Sync).
- **Notifications & look:** notifications, theme, terminal font.
- **System:** the home's path, listen, maintenance (images, the Podman machine).

**Done when:** the four tabs replace the two (the URL's `tab` follows; the old values open Agents); links that name a tab (`Settings → Workspace` in the session-limit toast, `Settings → Global → Profiles` across the UI and specs) are updated; the Settings spec describes it; UI tests cover the tab from the URL.

- [ ] the tabs and the "kept in" tags
- [ ] links to tabs across the UI, specs and README
- [ ] spec: Settings
