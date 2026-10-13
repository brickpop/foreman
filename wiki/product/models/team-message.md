---
type: spec
status: planned
tags: [model, team-tools, runtime]
---

# Team message

One message between two instances of the same [deployment](./deployment.md), routed by Foreman's [Team tools](../architecture/team-tools.md) router and stored in `state.db`. Shown in the [Sessions](../features/sessions.md) Messages drawer (this deployment only) and as marker lines (⇄) in the terminals.

## Schema

Table `team_messages`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | Integer | yes | auto | | |
| `deployment_id` | String → Deployment | yes | | | Team messaging is scoped to a deployment |
| `from_instance` | String → Instance | yes | | same deployment | |
| `to_instance` | String → Instance | yes | | same deployment | |
| `kind` | String | yes | | enum: `ask`, `reply`, `handoff` | One per team tool that sends a message. The PRD also listed `review`, but no tool produces it: a lead's review comments are an `ask` to the author |
| `request_ref` | String | | | `req-N` | Monotonically increasing **per deployment** (the PRD; the prototype used one global counter). An `ask` creates it; its `reply` carries the same ref. The same `req-41` can exist in two deployments, so the UI always shows it with its deployment |
| `body` | String | yes | | | For a reply, **only** the designated `output` |
| `artifact_ref` | String | | | a branch name or a wiki path | Optional artifact reference from `reply` or `handoff` |
| `created_at` | Timestamp | yes | now | | |
| `delivered_at` | Timestamp | | | | When the line reached the target's PTY. Empty while queued for a busy target ([Team tools](../architecture/team-tools.md#delivery)) |

## Invariants

- `from_instance` and `to_instance` belong to the same deployment. Cross-deployment messaging is deliberately disallowed.
- `req-N` numbers never repeat within a deployment.
- A reply carries only `output` (and the optional artifact reference); no other context crosses sandboxes.
