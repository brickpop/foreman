---
type: spec
status: built
tags: [feature, ui, messaging]
---

# Project activity

A project's **Activity** tab ([Wiki reader → Project header and tabs](./wiki-reader.md#project-header-and-tabs)): what its teams did there, their messages and git actions, across every deployment on the project. [Changes](./changes.md) stays the pipeline (needs you, in flight, merged); this tab is the traffic. Decided by the CEO on 2026-10-16 and 2026-10-17 ([Project → Activity tab](../../backlog/project-activity-tab.md)).

## The timeline

Body padding `24px 48px 64px`, the full width.

- **One timeline** of the project's active deployments, newest first, in one panel: the rows of [Sessions → Activity](./sessions.md#activity) (a thread per row, which opens into a chat; a git action per row on a faint tint), each starting with its team's name (`Atlas ·`, 11.5px `--tx4`, so the agents' labels stay the loudest). No "who calls whom" map: Sessions keeps it.
- **Team chips** (segmented: `All teams`, then one per active deployment) when more than one team is deployed. A team's chip shows that deployment alone.
- **50 items at a time**, then a **`Show more`** button while there are more. A thread whose ask is on a later page joins it once that page loads.
- A thread's **`Open <asker>'s pane`** opens [Sessions](./sessions.md) on that deployment with that agent's pane.
- Times are local: `14:05` today, `Oct 15 14:05` earlier this year, `Oct 15 2025 14:05` before.
- **Live:** new messages and actions (`team.message`, `team.action`) refetch the pages loaded.
- **Empty:** `No team is deployed on <project>. Deploy one from its Settings tab.`; a deployment with nothing yet: `Nothing yet.`

## Older sessions

Below the timeline, only when the project has stopped deployments: **`Older sessions · N`** (a section label with a chevron), collapsed. Opened, one line per stopped deployment: chevron, the team's name (600), `stopped <time> · <goal>` (`--tx4`). Each opens its own timeline, paged the same way, without team names (the line above says it) and without `Open … pane` (a stopped deployment has no sessions).

A redeployed pair reactivates its old row ([Deployment](../models/deployment.md)), so its earlier runs' traffic is in the timeline, not in Older sessions.

## Data

`GET /api/activity?project=<id>` (the active deployments, merged) or `?deployment=<team@project>` (one), with `&before=<cursor>` for the next page ([API](../architecture/api.md)): `{items: [{message} | {action}], next}`, 50 items, newest first; `next` is empty at the end. The store pages both tables in one query, ordered by the time (as `julianday`, since the stored RFC 3339 text with trimmed nanoseconds doesn't sort as text below a second), then the table, then the id. Older sessions come from `GET /api/deployments?status=stopped`.
