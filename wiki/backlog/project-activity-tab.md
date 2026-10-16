---
type: task
status: todo
priority: medium
tags: [feature]
epic: messaging-handoffs
---

# Project → Activity tab

**Why:** the CEO, 2026-10-16. The team's traffic (threads between agents, and their git actions) is only in Sessions' Activity panel, one deployment at a time. On a project, the CEO wants to see what happened there. Changes stays the pipeline (needs you, in flight, merged), a different thing ([Changes](../product/features/changes.md), [Sessions → Activity](../product/features/sessions.md)).

**Decided (the CEO, 2026-10-16):** its own tab, `Overview · Activity · Changes · Board · Settings`.

**Done when:** the tab shows the same threads and git actions as the Sessions panel (the same components and `app/activity.ts`), across every deployment on the project, newest first. Each item says which team, and opens the session it came from.

- [ ] spec (Wiki reader → tabs, a new Activity section); `GET /api/messages` and `/api/actions` by project, or the deployment's filter widened
- [ ] stopped deployments: shown, marked as stopped, or left out (to decide when the card starts)
- [ ] tests
