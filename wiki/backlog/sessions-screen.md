---
type: task
status: blocked
priority: high
tags: [feature]
---

# Wire Sessions and Home to the real backend

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** the screens exist on the demo client ([Home and Sessions screens](./ui-home-sessions.md)); this card swaps their data for the real server, area by area ([Sessions](../product/features/sessions.md), [Home](../product/features/home.md)).

Blocked by [Home and Sessions screens](./ui-home-sessions.md) and [Instance manager](./instance-manager.md).

**Done when:** deployments, instances, statuses and terminals come from the real server (endpoints and events), the demo client no longer serves these areas (one line each in `ui/src/main.tsx`), no "demo data" badge remains on them, and requests stay on the demo client until epic 4.

- [ ] endpoints and events for deployments, instances and statuses
- [ ] swap the areas in main.tsx; remove their badges
- [ ] exact snapshots on reload with real sessions; the busy-agent warning from real statuses
