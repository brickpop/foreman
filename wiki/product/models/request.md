---
type: spec
status: partial
tags: [model, requests, runtime]
---

# Request

Something that needs the CEO, stored in `state.db`. The kinds, who raises them, the CEO's actions and the resolution rules are in [Requests](../features/requests.md); this entry is the stored shape. Requests show on [Home](../features/home.md) and in the project's [Changes](../features/changes.md), and trigger [notifications](../features/notifications.md).

## Schema

Table `requests`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `kind` | String | yes | | enum: `approval`, `question`, `review`, `finding`, `blocked` | |
| `project_id` | String | yes | | | |
| `deployment_id` | String → Deployment | | | | Absent for Chief of Staff requests |
| `instance_id` | String → Instance | | | | Who raised it (or who it is about, for `blocked`). Absent for Chief of Staff requests |
| `summary` | String | yes | | | The text shown on the request panel |
| `options_json` | JSON array of String | | `[]` | | Choices; the first is the primary action |
| `source` | String | yes | | enum: `agent`, `harness`, `foreman`, `cos` | `agent` via `escalate` or `request_merge`; `harness` when the harness reports it is waiting for the user (approval or question, [Protocol](../architecture/protocol.md#waiting-for-the-ceo)); `foreman` for blocked; `cos` for Chief of Staff requests (rendered "Chief of Staff", no instance needed) |
| `status` | String | yes | `open` | enum: `open`, `resolved`, `expired` | |
| `answer` | String | | | | The chosen option or typed text |
| `created_at` | Timestamp | yes | now | | |
| `resolved_at` | Timestamp | | | | |
| `merge_request_id` | String → Merge request | | | | For `review` only: the [merge request](./merge-request.md) it reviews |
| `links_json` | JSON | | | | Extra links shown on the panel (e.g. a branch, a wiki entry) |

## Invariants

- Resolving writes the answer to the agent's stdin (and resumes a paused `escalate(approval)` tool call), sets `status: resolved`, `answer` and `resolved_at`.
- Resolved requests stay in `state.db` as history.
- Duplicates are merged: same instance, kind and summary within 10 minutes, comparing summaries normalized (case-folded, whitespace collapsed). Known limit: an agent rephrasing the same ask still creates a second request, and two different asks worded identically merge.
- Only `open` requests count toward "needs you".
- An instance can have **many open requests**, each its own request panel on Home and in Changes. Because `escalate(approval)` blocks the caller, one agent naturally waits on one approval at a time (a harness issuing parallel tool calls can still open several, which is allowed); questions and findings don't block and can pile up. The [instance](./instance.md) is `awaiting` while any blocking approval of it is open.
- The panel's **source label** is derived, not stored: `escalate(<kind>)` for `source: agent` on approval, question and finding; `request_merge()` for review; `waiting in its session` for `source: harness`; `raised by Foreman` for `source: foreman`; `Chief of Staff` for `source: cos`.

## Phasing

Built (2026-10-17, ahead of the Workflow epic, at the CEO's request: a merge request waiting for the CEO must show on Home): the table and `GET /api/requests` (the open ones), with `request.created` and `request.resolved`; **review** requests only. `request_merge` raises one per merge request (a refresh raises no second), and the merge request leaving `open` resolves it (`answer`: `merged` or `rejected`); one made before reviews existed gets its review at the next reconcile. Home and Changes decide it with the review panel. Not yet: `escalate` and the other kinds, answering (`POST /api/requests/:id/resolve`), `harness` and `foreman` sources, duplicate merging.
