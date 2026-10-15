---
type: task
status: todo
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Wire Sessions and Home to the real backend

**Why:** the screens exist ([Home and Sessions screens](./ui-home-sessions.md)) and run on the real server since the UI went live; this card finishes what they need from it ([Sessions](../product/features/sessions.md), [Home](../product/features/home.md)).

**Done when:** deployments, instances, statuses and terminals come from the real server (endpoints and events), with their last lines and exact snapshots; requests say "Not built yet" until epic 4.

- [x] endpoints and events for deployments, instances and statuses
- [ ] the `instance.line` event (a throttled last output line, ANSI stripped), shown on Project settings rows
- [x] the areas on the real client, no badges (the UI went live, [Full UI demo](./epics/full-ui-demo.md#going-live))
- [ ] exact snapshots on reload with real sessions
