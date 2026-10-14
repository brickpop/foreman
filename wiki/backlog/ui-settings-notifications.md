---
type: task
status: todo
priority: medium
tags: [feature]
epic: full-ui-demo
---

# Settings screen, notifications and toasts

**Why:** workspace settings and how the CEO hears from agents ([Workspace settings screen](../product/features/settings.md), [Notifications](../product/features/notifications.md), [App shell](../product/features/app-shell.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** every Settings section (Workspace with Listen on and the engine choice, Harnesses with Connect and the login dialog, Forge accounts with the token form, Notifications, Maintenance with the image chain, Refresh, Prune and Log, Appearance); browser notifications (permission, toggles, tags, click to open the session) and toasts.

- [ ] Settings sections
- [ ] Maintenance on the real server: an API for the image status and build log (`runner.Images.Status`), the Podman version and Rebuild (`Refresh`), from the [Image builder](./image-builder.md)
- [ ] the Connect dialog (a terminal on the login container; demo on the demo client)
- [ ] browser notifications and toasts
