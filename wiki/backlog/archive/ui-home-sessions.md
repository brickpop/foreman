---
type: task
status: done
priority: high
tags: [feature]
epic: full-ui-demo
---

# Home and Sessions screens

**Why:** the CEO's daily view: what needs them, and the agents' terminals ([Home](../../product/features/dashboard.md), [Sessions](../../product/features/sessions.md), [Requests](../../product/features/requests.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** Home shows the request panels (all kinds, their actions, the order), the projects list, and the empty state; Sessions shows deployment tabs, agent tabs (lead first, Direct divider, status dots), single or split panes with headers (Copy, close), the direct-line banner, line-input mode, the markers strip and the Messages drawer. The terminal is real (the real client); the rest runs on demo data.

- [x] Home: request panels (the shared `RequestPanel`, also for Changes: every kind, options by position, free-text answer, Review → Changes, Open session → the agent's pane, Chief of Staff items), the order, the empty state, projects list, title and subtitle
- [x] Sessions: deployment tabs (needs-you dots), agent tabs (lead first, Direct divider, status dots, underlines), one or two panes, Split/Single, active pane, close; panes remembered per deployment while the app runs (across browsers: with the `ui_prefs` API)
- [x] direct-line banner, line-input mode (the `Line` button; sent through the terminal connection)
- [x] markers strip (for now derived from the team messages; `instance.marker` events replace it with Messaging) and the Messages drawer (newest first, click opens the sender)
- [x] the real Terminal component in the panes (each demo agent shows the server's demo session)
- [x] the CEO looks at it on the Mac (`just dev`): approved 2026-10-15
