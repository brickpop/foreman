---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: full-ui-demo
---

# Settings screen, notifications and toasts

**Why:** workspace settings and how the CEO hears from agents ([Workspace settings screen](../product/features/settings.md), [Notifications](../product/features/notifications.md), [App shell](../product/features/app-shell.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** every Settings section (Workspace with Listen on and the engine choice, Harnesses with Connect and the login dialog, Forge accounts with the token form, Notifications, Maintenance phase 1 with the image, its log and Rebuild, Appearance); browser notifications (permission, toggles, tags, click to open the session) and toasts. Built; waiting for the CEO's look on the Mac.

- [x] Settings sections: Workspace (Listen on and Run agents in confirm before weakening safety, with a warning while Direct is on; sessions stepper; markers switch), Harnesses (command input, placeholder = the adapter's default; login state; Connect dialog), Forge accounts (inline host + write-only token form; the workspace repo's forge with its URL, Sync disabled until the [Git epic](./epics/git.md)), Notifications, Maintenance, Appearance
- [x] `PUT /api/settings` (version-guarded: 409 / 400 with the problems)
- [x] Maintenance on the real server: `GET /api/maintenance` (engine state, image status and build log) and `POST /api/maintenance/rebuild` (`Images.Refresh`, in the background; the screen polls while building). The UI serves it from the demo client like every other area (one mode until the review of the whole UI; the demo engine line follows the setting)
- [x] the Connect dialog (a terminal on `connect:<harness>`; the demo session on the demo client)
- [x] browser notifications and toasts: `noticeFor` (request.created → needs me / merge requested; instance done → finishes), a toast always, a browser notification when permission, toggle and a hidden tab allow; tagged per instance; a click opens the session
- [x] tests: `PUT /api/settings`, the maintenance routes and engine line, the demo client's settings/forge/rebuild, the notices and their gating

Deferred, tracked elsewhere: forge tokens on the server ([Git epic](./epics/git.md)); build time and harness versions ([Full image chain](./image-chain.md)); the theme across browsers ([technical debt](../technical-debt.md#the-theme-is-kept-in-the-browser-only)).
