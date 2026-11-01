---
type: spec
status: planned
tags: [feature, ui, requests]
---

# Notifications

Foreman tells the CEO when something happens, so they don't have to watch the UI. Two channels, fed by the same server events ([API](../architecture/api.md), `GET /api/events`):

- **Browser notifications** (Notification API), after explicit permission granted from [Settings](./settings.md).
- **In-app toasts**, always, regardless of browser permission ([App shell](./app-shell.md#toasts)).

## When they fire

| Event | Setting toggle | Title / body |
|---|---|---|
| A new [request](./requests.md), any kind | "An agent needs me" | `<Name> needs you on <project>` / the summary |
| An instance becomes `done` ([Instance](../models/instance.md)) | "An agent finishes" | `<Name> is done on <project>` / the handoff summary |
| A review request is raised (a merge is requested) | "A merge is requested" | `<Lead> requested a merge on <project>` / the entry's title |
| A review is withdrawn, by its lead or by Foreman ([Reviews → Withdrawn](./reviews.md#withdrawn)) | none: a toast only (nothing waits on the CEO) | `<Lead> withdrew <branch>` (or `Foreman withdrew <branch>`) / `<reason>.`, then ` Your <n> draft comments on it are lost.` when any were; the dot is `--err` then, else `--tx4`. Its click opens the review |
| An instance becomes `queued`: every session slot is taken, idle agents included (the CEO, 2026-10-18) | none: a toast only | `<Name> is waiting for a session slot` / `Every slot is taken, idle agents included. They start when one frees; the limit is in Settings → Agents.` Its click opens the agent's pane, which says the same with a link to Settings |
| Foreman stops a deployment on its own, its project gone ([Deployment](../models/deployment.md)) | none: a toast only | `Foreman stopped <Team> on <project>` / the reason (`<project> is gone: all its code repos are missing`, or `…: its project repo, <path>, is gone`). The CEO's own Stop says nothing here |

The toggles gate browser notifications only; toasts always show. A tab raises a browser notification only while it is hidden (`document.hidden`); while the CEO is looking at it, its toast says it already.

A Foreman-raised request (no instance, e.g. a failed build) is titled `Foreman needs you on <project>`. The done notice's body is a fixed line until instance events carry the hand-off summary.

Not built: notifications from the OS rather than the browser, chosen in Settings ([its card](../../backlog/os-notifications.md)).

## Behaviour

- Each notification is tagged per instance (`agent@project`), so a newer one replaces an older one from the same instance. The same tag also collapses duplicates when several Foreman tabs are open (every tab receives the event; the browser shows one notification per tag). To check in every browser in scope.
- Clicking a notification (or its toast) focuses the Foreman tab and opens that agent in its project's [Session](./sessions.md); a review's opens the review's page in its project's [Inbox](./reviews.md#the-inbox) instead (the CEO, 2026-10-21).
- Notifications are raised by any open Foreman tab; with several tabs open, only one shows each notification.
