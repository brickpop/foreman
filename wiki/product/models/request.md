---
type: spec
status: planned
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
| `summary` | String | yes | | | The text shown on the card |
| `options_json` | JSON array of String | | `[]` | | Choices; the first is the primary action |
| `source` | String | yes | | enum: `agent`, `foreman` | `agent` via `escalate` or `open_pr`; `foreman` for blocked and Chief of Staff reviews |
| `status` | String | yes | `open` | enum: `open`, `resolved`, `expired` | |
| `answer` | String | | | | The chosen option or typed text |
| `created_at` | Timestamp | yes | now | | |
| `resolved_at` | Timestamp | | | | |
| `links_json` | JSON | | | | e.g. PR URL, branch, pipeline item, forge kind |

## Invariants

- Resolving writes the answer to the agent's stdin (and resumes a paused `escalate(approval)` tool call), sets `status: resolved`, `answer` and `resolved_at`.
- Resolved requests stay in `state.db` as history.
- Duplicates are merged: same instance, kind and summary within 10 minutes.
- Only `open` requests count toward "needs you".

## Open questions

1. **Duplicate detection by summary text.** Exact text equality is fragile: an agent rephrasing the same ask creates a duplicate, and two genuinely different asks with the same wording merge. Options: exact match (PRD); normalized match (case, whitespace); a client-supplied dedupe key in `escalate`. Recommendation: normalized match in v1, and record it as a known limit.
2. **Chief of Staff requests.** The prototype's CoS config reviews have no instance or deployment (`cos@<project>`), and a source shown as "Chief of Staff". Recommendation: add `source` value `cos`, so the UI can render "Chief of Staff" and the Changes card's source line without guessing.
3. **One or several open approvals per instance?** The prototype allows at most one open approval per instance; the PRD doesn't say. Since an approval pauses the agent, a second normally can't happen, but a harness may issue parallel tool calls. Options: enforce one open approval per instance, a second `escalate(approval)` returns an error to the agent ([Requests](../features/requests.md)); or allow several, with the instance staying `awaiting` until all blocking ones are resolved. No single recommendation (the two drafts disagreed). UI implications either way: Home and Changes show one card per approval, and the agent's status dot stays `awaiting` while any is open.
4. **Source label on cards.** The prototype's Changes cards show `escalate(approval)`, `open_pr()`, `raised by Foreman`, `Chief of Staff`. Derivable from `kind` + `source`; confirm no extra field is needed.
