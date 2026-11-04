---
type: spec
status: partial
tags: [model, requests, runtime]
---

# Request

Something that needs the CEO, stored in `state.db`. The kinds, who raises them, the CEO's actions and the resolution rules are in [Requests](../features/requests.md); this entry is the stored shape. Requests show on [Dashboard](../features/dashboard.md) and on the project's Overview, under [Needs you](../features/project-pipeline.md#1-needs-you), and trigger [notifications](../features/notifications.md).

## Schema

Table `requests`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `kind` | String | yes | | enum: `approval`, `question`, `review`, `finding`, `blocked` | |
| `project_id` | String | yes | | | |
| `deployment_id` | String → Deployment | | | | |
| `instance_id` | String → Instance | | | | Who raised it (or who it is about, for `blocked`) |
| `summary` | String | yes | | one line, at most 120 characters | The request's title, shown first on its panel and in its notification |
| `body` | String | yes | `''` | at most 8,000 characters | The detail under the title, when it needs more; rendered as Markdown (GitHub's, no raw HTML, links opening in a new tab), folded past 6 lines or 480 characters |
| `options_json` | JSON array of String | | `[]` | | Choices; the first is the primary action |
| `source` | String | yes | | enum: `agent`, `harness`, `foreman` | `agent` via `escalate` or `request_merge`; `harness` when the harness reports it is waiting for the user (approval or question, [Protocol](../architecture/protocol.md#waiting-for-the-ceo)); `foreman` for blocked |
| `status` | String | yes | `open` | enum: `open`, `resolved`, `expired`, `dismissed` | |
| `answer` | String | | | | The chosen option or typed text |
| `created_at` | Timestamp | yes | now | | |
| `resolved_at` | Timestamp | | | | |
| `review_id` | String → Review | | | | For `review` only: the [review](./review.md) it asks for (its branch, every repo it changed) |
| `links_json` | JSON | | | | Extra links shown on the panel (e.g. a branch, a wiki entry) |

## Invariants

- Resolving an escalation sends the answer to the lead first, and only once the message is kept does the request close (`status: resolved`, `answer`, `resolved_at`): an answer that couldn't be kept is an error, and the request stays open to answer again (2026-11-04: answers were lost behind a resolved request).
- **Dismissing** closes it unanswered (`status: dismissed`), from any source but a review; an escalation of a live deployment first tells its lead ([Requests → Rules](../features/requests.md#rules)).
- A request ends once: whichever way comes second (the CEO dismissing it, then its agent's activity resolving it) finds it closed and changes nothing.
- Resolved requests stay in `state.db` as history.
- Duplicates are merged: same instance, kind, summary and body within 10 minutes, comparing them normalized (case-folded, whitespace collapsed). Known limit: an agent rephrasing the same ask still creates a second request, and two different asks worded identically merge.
- Only `open` requests count toward "needs you".
- An instance can have **many open requests**, each its own request panel on the Dashboard and in the project's Inbox. Because an approval has the lead wait for the answer, it naturally waits on one approval at a time (a harness issuing parallel tool calls can still open several, which is allowed); questions and findings don't block and can pile up. An open request doesn't change the [instance](./instance.md)'s status (messages keep flowing to it); the agent's rows show it waits on the CEO from the request itself. `awaiting` is the harness waiting at its own dialog.
- The panel's **source label** is derived, not stored: `escalate(<kind>)` for `source: agent` on approval, question and finding; `request_merge()` for review; `waiting in its session` for `source: harness`; `raised by Foreman` for `source: foreman`.

## Phasing

Built (2026-10-17, ahead of the Workflow epic, at the CEO's request: a merge request waiting for the CEO must show on the Dashboard): the table and `GET /api/requests` (the open ones), with `request.created` and `request.resolved`; **review** requests only. Since 2026-10-21 a review request is one per [review](./review.md), not per merge request: `request_merge` raises it while the review needs the CEO (a refresh raises no second), Request changes resolves it (`changes requested`) until the lead's next `request_merge`, and the review having nothing open resolves it (`merged`, `dismissed` or `withdrawn`); one whose request couldn't be saved (a database error) gets it at the next reconcile. Its summary is `Merge <branch>: <title>`, the review's title when raised (a refresh doesn't change it: lists show the review's own title, so only its notification reads it); the Dashboard and the project's Overview (Needs you) show it with a **Review** button that opens its page ([Reviews](../features/reviews.md)). **Blocked** requests (`source: foreman`) are raised for a stuck agent (`internal/instances/stuck.go`) and resolved by its next activity or a restart. Since 2026-11-04: `escalate` and its kinds (approval, question, finding; the lead only), answering them (`POST /api/requests/:id/resolve`, sent to the lead as an `answer` message), `expired` when their deployment stops, and duplicate merging for escalations; since 2026-11-04 too, a short title (`summary`) and an optional `body` instead of one summary carrying it all (the CEO: "not just a single title that must explain everything"); the `harness` source, a question card raised when the harness waits at its own dialog and resolved by its next activity ([Waiting in its session](../../backlog/harness-waiting-requests.md); the CEO's pick isn't typed into the dialog yet). Since 2026-11-04 as well, dismissing any card but a review. Not yet: crash-blocked requests.
