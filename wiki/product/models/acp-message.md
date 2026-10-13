---
type: spec
status: planned
tags: [model, acp, runtime]
---

# ACP message

One message between two instances of the same [deployment](./deployment.md), routed by Foreman's [ACP](../architecture/acp.md) router and stored in `state.db`. Shown in the [Sessions](../features/sessions.md) Messages drawer (this deployment only) and as marker lines (⇄) in the terminals.

## Schema

Table `acp_messages`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | Integer | yes | auto | | |
| `deployment_id` | String → Deployment | yes | | | ACP is scoped to a deployment |
| `from_instance` | String → Instance | yes | | same deployment | |
| `to_instance` | String → Instance | yes | | same deployment | |
| `kind` | String | yes | | enum: `ask`, `reply`, `handoff`, `review` | |
| `request_ref` | String | | | `req-N` | Monotonically increasing per deployment. An `ask` creates it; its `reply` carries the same ref |
| `body` | String | yes | | | For a reply, **only** the designated `output` |
| `artifact_ref` | String | | | | Optional artifact reference from `reply` or `handoff` (e.g. a branch) |
| `created_at` | Timestamp | yes | now | | |

## Invariants

- `from_instance` and `to_instance` belong to the same deployment. Cross-deployment messaging is deliberately disallowed.
- `req-N` numbers never repeat within a deployment.
- A reply carries only `output` (and the optional artifact reference); no other context crosses sandboxes.

## Open questions

1. **`review` kind.** The PRD lists `review` but no ACP tool produces it (the tools are `ask`, `reply`, `handoff`, `escalate`, `open_pr`; see [ACP](../architecture/acp.md)), and the prototype only shows ask/reply/handoff. Is it the lead's review comments or send-back to an author (prototype line "Mara sent 2 review comments"), or should it be dropped? Recommendation: drop it; review comments are an `ask` to the author.
2. **`req-N` scope.** The PRD numbers per deployment; the prototype uses one global counter. Per deployment means the same `req-41` can exist in two deployments, so UI links must always qualify it with the deployment. Recommendation: keep per deployment (PRD).
3. **Delivery state.** A message to a busy target is queued until its next idle prompt. The table has no field recording queued vs delivered. Recommendation: add `delivered_at`.
