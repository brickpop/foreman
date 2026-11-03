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
| `summary` | String | yes | | | The text shown on the request panel |
| `options_json` | JSON array of String | | `[]` | | Choices; the first is the primary action |
| `source` | String | yes | | enum: `agent`, `harness`, `foreman` | `agent` via `escalate` or `request_merge`; `harness` when the harness reports it is waiting for the user (approval or question, [Protocol](../architecture/protocol.md#waiting-for-the-ceo)); `foreman` for blocked |
| `status` | String | yes | `open` | enum: `open`, `resolved`, `expired` | |
| `answer` | String | | | | The chosen option or typed text |
| `created_at` | Timestamp | yes | now | | |
| `resolved_at` | Timestamp | | | | |
| `review_id` | String → Review | | | | For `review` only: the [review](./review.md) it asks for (its branch, every repo it changed) |
| `links_json` | JSON | | | | Extra links shown on the panel (e.g. a branch, a wiki entry) |

## Invariants

- Resolving delivers the answer into the agent's session as one line ([Requests → Rules](../features/requests.md#rules)), sets `status: resolved`, `answer` and `resolved_at`.
- Resolved requests stay in `state.db` as history.
- Duplicates are merged: same instance, kind and summary within 10 minutes, comparing summaries normalized (case-folded, whitespace collapsed). Known limit: an agent rephrasing the same ask still creates a second request, and two different asks worded identically merge.
- Only `open` requests count toward "needs you".
- An instance can have **many open requests**, each its own request panel on the Dashboard and in the project's Inbox. Because an approval has the lead wait for the answer, it naturally waits on one approval at a time (a harness issuing parallel tool calls can still open several, which is allowed); questions and findings don't block and can pile up. The [instance](./instance.md) is `awaiting` while any blocking approval of it is open.
- The panel's **source label** is derived, not stored: `escalate(<kind>)` for `source: agent` on approval, question and finding; `request_merge()` for review; `waiting in its session` for `source: harness`; `raised by Foreman` for `source: foreman`.

## Phasing

Built (2026-10-17, ahead of the Workflow epic, at the CEO's request: a merge request waiting for the CEO must show on the Dashboard): the table and `GET /api/requests` (the open ones), with `request.created` and `request.resolved`; **review** requests only. Since 2026-10-21 a review request is one per [review](./review.md), not per merge request: `request_merge` raises it while the review needs the CEO (a refresh raises no second), Request changes resolves it (`changes requested`) until the lead's next `request_merge`, and the review having nothing open resolves it (`merged`, `dismissed` or `withdrawn`); one whose request couldn't be saved (a database error) gets it at the next reconcile. Its summary is `Merge <branch>: <entry title>.`; the Dashboard and the project's Overview (Needs you) show it with a **Review** button that opens its page ([Reviews](../features/reviews.md)). **Blocked** requests (`source: foreman`) are raised for a stuck agent (`internal/instances/stuck.go`) and resolved by its next activity or a restart. Not yet: `escalate` and its kinds ([Escalate](../../backlog/escalate.md)), answering (`POST /api/requests/:id/resolve`), the `harness` source ([Waiting in its session](../../backlog/harness-waiting-requests.md)), crash-blocked requests, duplicate merging.
