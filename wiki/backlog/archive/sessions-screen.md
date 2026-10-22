---
type: task
status: done
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Wire Sessions and Dashboard to the real backend

**Why:** the screens exist ([Dashboard and Sessions screens](./ui-home-sessions.md)) and run on the real server since the UI went live; this card finishes what they need from it ([Sessions](../../product/features/sessions.md), [Dashboard](../../product/features/dashboard.md)).

**Done when:** deployments, instances, statuses and terminals come from the real server (endpoints and events), with their activity lines and exact snapshots; requests say "Not built yet" until epic 4.

- [x] endpoints and events for deployments, instances and statuses
- [x] each agent's activity line on Project settings rows, from its harness's hooks (`instance.doing`, built 2026-10-18; the CEO chose hooks over the terminal's last line, which for Claude Code is its footer)
- [x] the areas on the real client, no badges (the UI went live, [Full UI demo](../epics/full-ui-demo.md#going-live))
- [x] exact snapshots on reload with real sessions: moved to [Terminal fidelity](../terminal-fidelity.md), its own card (2026-10-22)
