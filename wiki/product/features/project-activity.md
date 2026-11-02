---
type: spec
status: built
tags: [feature, ui, messaging]
---

# Project activity

A project's **Activity** section ([Project page](./project-page.md)): what its team did there, its messages and git actions, with the map of who calls whom; then the earlier teams' traffic. The [project pipeline](./project-pipeline.md) (what needs the CEO in the Inbox, in flight and merged on [Deployment](./project-deployment.md)) is what moves towards the targets; this section is the traffic. Decided by the CEO on 2026-10-16 and 2026-10-17 ([Project → Activity tab](../../backlog/archive/project-activity-tab.md)); on 2026-10-31 the Activity panel beside the session's terminal moved here, map and all, since with one team per project it showed the same traffic as this section.

The terminal shows only what the agent's program prints. Anything Foreman wrote into it would be typed input to the agent ([Protocol → Deliveries and markers](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)), so the team's activity is shown here, never in a terminal (the CEO's decision, 2026-10-16).

## Layout

Padding `24px 24px 48px`. The **timeline** on the left, taking the width; the **map** on the right (a panel, 328px wide), staying in view while the timeline scrolls. Under 1100px the map goes above the timeline, full width. Below both, **Older sessions**. With no team on the project: `No team is on <project>. Deploy one from its Deployment section.`, and Older sessions.

## The map

**Who calls whom** (260px tall, 150px until the CEO asked for it taller on 2026-11-02): a name box per agent on an ellipse (radii 118 by 100px) (in the primary colour, filled `--acc` when it is the filter), with its status dot (and the word `queued` when it waits for a session slot: the dashed dot is too small to tell from idle; the box's tooltip names the status), plus the CEO when they sent something. Foreman is never on the map (the CEO, 2026-10-17): its notices (a merge done, a conflict, a target moving) are system news, not teamwork, so they show in the list only. A line joins each pair that exchanged messages, thicker with more of them, `--line3`, or `--acc` while a message between them waits; at its middle a small count badge (Mono 10.5px) whose tooltip gives both directions. Clicking a name box filters the timeline to its messages and actions; clicking a badge, to that pair's messages (an action is one agent's, so a pair leaves them out); again, or `Show all` in the strip above the timeline that names the filter, shows everything.

The map draws the whole of the team's messages (`GET /api/messages?deployment=`), while the timeline is paged: a filter applies to the pages loaded, so a filtered timeline can say `Nothing here in what's loaded.` with `Show more` still offered.

## The timeline

One timeline of the team's deployment, newest first, in one panel:

- **Names are labels** everywhere in the timeline, in the primary colour: a small box (`--accSoft`, 1px `--accLine`, radius 5, 12px 600, `--accTx`); the CEO's (`You`, `--raised`, `--tx`) and Foreman's (muted) stand apart.
- **An exchange reads `<agent> <verb> <agent>`:** the verb, in the past tense and its kind's colour, needs no box and no arrow (it says the direction): `asked` (`--info`), `replied to` (`--ok`), `handed off to` (`--tx2`), `wrote to` and `notified` (`--tx3`). Names keep their labels.
- **The exchanges, newest activity first, one row per thread** (an ask and its replies; a hand-off, a contribution or a notice on its own): the exchange (`Tic asked Tac`), a spacer, how it stands (`no reply yet`, `1 reply`, `waiting` in `--attnTx`, `expired`, `failed`), the time of its last message (local: `14:05` today, `Oct 15 14:05` before); under it the first message's start, on one line. A click opens the thread as a chat: each message a bubble (the asker's on the left, `--raised`; the other side's on the right, `--panel`), with its sender's label, `kind · msg-N`, the time, the whole text (selectable; its `inline code` and ``` fenced blocks ``` shown as code, the rest as plain text, never HTML), the artifact, `Waiting: <why> (the recipient's state: not started because every session slot is taken, idle agents included, with the count; busy until its turn ends; waiting for the CEO; asleep and being woken; the CEO's unsent line in its terminal), the row's `waiting` carrying the same as its tooltip` or a failure's reason; then `Open <asker>'s session`.
- **Git actions** (`switch`, `commit`, `merge`, `abort_merge`, `discard`, `set_submodule` (its commit message, which says what it did), and the lead's `request_merge`, which names each merge request it made and links **Review it**, which opens the project's Inbox; [Protocol → Team actions](../architecture/protocol.md#team-actions)) sit between the threads, one row each, newest first, on a faint `--side` tint: the agent's label and what it did (`committed "cal v0"` for one repo, `committed api, project` for several, each repo's line then ending with its own message; `switched to wt/tac/LED-44 (new)`, `merged wt/tac/main`, `aborted a merge`; `tried to …` and `refused` in `--err` when refused), the time; under it each repo (Mono 11.5px: the repo, its branch, the commit's 7 characters, conflicts in `--accTx`), or the refusal's text. Filtering by an agent shows its actions; filtering by a pair leaves them out.
- **50 items at a time**, then a **`Show more`** button while there are more. A thread whose ask is on a later page joins it once that page loads.
- A thread's **`Open <asker>'s session`** opens the [Session](./sessions.md) section on that agent.
- Times are local: `14:05` today, `Oct 15 14:05` earlier this year, `Oct 15 2025 14:05` before.
- **Live:** new messages and actions (`team.message`, `team.action`) refetch the pages loaded, and the map.
- **Empty:** `Nothing yet.`

What an agent itself receives (a teammate's ask, a reply, the CEO's answer) is a **delivery**: typed into its session when it is idle, so it appears in the terminal as the harness shows any input. The texts are in [Protocol → Team tools](../architecture/protocol.md#team-tools).

**Later:** Foreman's other notes for the CEO (`instance.marker` events: escalations, answers, expiries, system notes, with the glyphs in [Team tools → Markers](../architecture/team-tools.md#markers-notes-for-the-ceo-never-in-the-terminal)) join the list as rows of their own.

## Older sessions

Below the timeline, only when the project has stopped deployments: **`Older sessions · N`** (a section label with a chevron), collapsed. Opened, one line per stopped deployment: chevron, the team's name (600), `stopped <time>` (`--tx4`), then, when Foreman stopped it on its own, `by Foreman: <reason>` (`--err`, [Deployment](../models/deployment.md)). Each opens its own timeline, paged the same way, its agents named as that team has them (not as a later team with the same ids does), without team names (the line above says it) and without `Open … session` (a stopped deployment has no sessions).

A redeployed pair reactivates its old row ([Deployment](../models/deployment.md)), so its earlier runs' traffic is in the timeline, not in Older sessions.

## Data

`GET /api/activity?deployment=<team@project>`, with `&before=<cursor>` for the next page ([API](../architecture/api.md)): `{items: [{message} | {action}], next}`, 50 items, newest first; `next` is empty at the end. The store pages both tables in one query, ordered by the time (as `julianday`, since the stored RFC 3339 text with trimmed nanoseconds doesn't sort as text below a second), then the table, then the id. The team on the project is its active deployment (`GET /api/deployments`); older sessions come from `GET /api/deployments?status=stopped`.
