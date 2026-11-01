---
type: spec
status: planned
tags: [model, team-tools, runtime]
---

# Team message

One message between two instances of the same [deployment](./deployment.md), routed by Foreman's [Team tools](../architecture/team-tools.md) router and stored in `state.db`. Shown in the project's [Activity](../features/project-activity.md#the-timeline), never in the terminals.

## Schema

Table `team_messages`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | Integer | yes | auto | | |
| `deployment_id` | String → Deployment | yes | | | Team messaging is scoped to a deployment |
| `from_instance` | String → Instance | yes | | same deployment, or `foreman` / `ceo` | `foreman` and `ceo` for the notices Foreman originates and the CEO's answers ([Protocol → Deliveries Foreman originates](../architecture/protocol.md#deliveries-foreman-originates)) |
| `to_instance` | String → Instance | yes | | same deployment | |
| `kind` | String | yes | | enum: `ask`, `reply`, `handoff`, `contribute`, `notice` | One per team tool that sends a message (`contribute`: the notice to the lead, its artifact the inbox entry); `notice`: from Foreman or the CEO. The PRD also listed `review`, but no tool produces it: a lead's review comments are an `ask` to the author |
| `ref` | String | yes | | `msg-N` | Every message's id within its deployment, monotonically increasing **per deployment** (the PRD; the prototype used one global counter). `read_message` reads a message by it, and `reply` answers an ask by it (decided 2026-10-15: one id per message, where the PRD's `req-N` numbered asks only). The same `msg-41` can exist in two deployments, so the UI always shows it with its deployment |
| `in_reply_to` | String | | | an ask's `ref` | A reply's ask |
| `body` | String | yes | | | For a reply, **only** the designated `output` |
| `artifact_ref` | String | | | a branch name or a wiki path | Optional artifact reference from `reply`, `handoff` or `contribute` |
| `created_at` | Timestamp | yes | now | | |
| `delivered_at` | Timestamp | | | | When its notice reached the target's PTY. Empty while queued for a busy target ([Team tools](../architecture/team-tools.md#delivery)) |
| `state` | String | yes | `queued` | enum: `queued`, `delivered`, `expired`, `failed` | [Protocol → Messages](../architecture/protocol.md#messages) |
| `reason` | String | | | | Why it expired or failed |

## Invariants

- `from_instance` and `to_instance` belong to the same deployment. Cross-deployment messaging is deliberately disallowed.
- `msg-N` numbers never repeat within a deployment.
- A reply carries only `output` (and the optional artifact reference); no other context crosses sandboxes.
