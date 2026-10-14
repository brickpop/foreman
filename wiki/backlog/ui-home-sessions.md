---
type: task
status: blocked
priority: high
tags: [feature]
---

# Home and Sessions screens

Epic: [Full UI on a demo client](./epics/full-ui-demo.md)

**Why:** the CEO's daily view: what needs them, and the agents' terminals ([Home](../product/features/home.md), [Sessions](../product/features/sessions.md), [Requests](../product/features/requests.md)).

Blocked by [Demo client](./demo-client.md) and [App shell](./app-shell.md).

**Done when:** Home shows the request panels (all kinds, their actions, the order), the projects list, and the empty state; Sessions shows deployment tabs, agent tabs (lead first, Direct divider, status dots), single or split panes with headers (Copy, close), the direct-line banner, the "another agent is busy" warning, line-input mode, the markers strip and the Messages drawer. The terminal is real (the real client); the rest runs on demo data.

- [ ] Home: request panels with their actions, projects list, title and subtitle
- [ ] Sessions: tabs, panes, split and active pane, persistence per deployment
- [ ] banner, busy warning, line-input mode
- [ ] markers strip and Messages drawer
- [ ] the real Terminal component in the panes
