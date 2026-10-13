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
| A review request is raised (PR ready) | "A PR is ready to merge" | `<Lead> opened a PR on <project>` / the PR title |

The toggles gate browser notifications only; toasts always show.

## Behaviour

- Each notification is tagged per instance (`agent@project`), so a newer one replaces an older one from the same instance. The same tag also collapses duplicates when several Foreman tabs are open (every tab receives the event; the browser shows one notification per tag). To check in every browser in scope.
- Clicking a notification focuses the Foreman tab and opens that instance's session.
- Notifications are raised by any open Foreman tab; with several tabs open, only one shows each notification.
