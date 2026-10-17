---
type: task
status: done
priority: medium
tags: [feature]
epic: workflow
---

# Project → Activity tab

**Why:** the CEO, 2026-10-16. The team's traffic (threads between agents, and their git actions) is only in Sessions' Activity panel, one deployment at a time. On a project, the CEO wants to see what happened there. Changes stays the pipeline (needs you, in flight, merged), a different thing ([Changes](../product/features/project-pipeline.md), [Sessions → Activity](../product/features/sessions.md)).

**Decided (the CEO, 2026-10-16):** its own tab, `Overview · Activity · Changes · Board · Settings`.

**Done when:** the tab shows the same threads and git actions as the Sessions panel (the same components and `app/activity.ts`), across every deployment on the project, newest first. Each item says which team, and opens the session it came from.

**Decided (the CEO, 2026-10-17):**
- **One timeline, full width**, threads and git actions together, newest first, each row naming its team. A row of team chips filters when more than one team is active. No "who calls whom" map: Sessions keeps it.
- **Active deployments** fill the timeline, **50 items at a time**, then **Show more**.
- **Stopped deployments** sit below, in a separate collapsed list, **Older sessions**: one row per stopped deployment; opening one shows its own timeline, paged the same way.

**Design (2026-10-17):**
- `GET /api/activity?project=<id>` (the project's active deployments, merged) or `?deployment=<id>` (one, for an older session), `&before=<cursor>`: `{items: [{message} | {action}], next}`, 50 per page, newest first; `next` is an opaque cursor, empty at the end. The store pages both tables in one query, ordered by `julianday(created_at)` then table then id: the stored time text (RFC 3339 with trimmed nanoseconds) doesn't sort as text below a second.
- `GET /api/deployments?status=stopped` lists stopped deployments (default: active, as today); the manager's `ActiveDeployments` becomes `Deployments(ctx, status)`.
- The UI reuses the Activity panel's rows (exported, with a team label), and `app/activity.ts`, whose threads are now keyed by deployment and ref (a `msg-N` is unique only within its deployment). Rows show local time, with the date when not today (the panel showed UTC: fixed for both).
- A row of an active deployment opens its session (Sessions on that deployment); an older session's threads open inline only.
- Live: `team.message` and `team.action` refetch the loaded pages.

- [x] store `Activity.Page`, its tests (order across both tables, the cursor, sub-second times, a deployment filter)
- [x] `/api/activity`, `/api/deployments?status=stopped`; server tests
- [x] UI: client, queries, threads keyed by deployment, local times, the tab (chips, Show more, Older sessions); tests of the pure parts
- [x] specs: Wiki reader (tabs), a new Project activity spec, API, Sessions (local times)

**Built 2026-10-17** ([Project activity](../product/features/project-activity.md)). Building it showed three flaws in the Sessions panel's code, fixed for both: threads were keyed by `msg-N` alone (two teams' `msg-1` would have merged), threading assumed messages oldest first (a page comes newest first, so a reply before its ask stayed alone), and times were shown in UTC (`createdAt.slice(11, 16)`; Changes' merged times too). The manager's `ActiveDeployments` became `DeploymentsIn(ctx, status)`.
