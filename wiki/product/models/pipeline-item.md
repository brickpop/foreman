---
type: spec
status: planned
tags: [model, pipeline, git, forge]
---

# Pipeline item

One unit of work tracked from a member's worktree to a PR on `main`, stored in `state.db`. Stages and the CEO's actions are in [Pipeline and pull requests](../features/pipeline-and-prs.md); items appear in the project's [Changes](../features/changes.md) (In flight and Merged), and a stage-4 item raises a review [request](./request.md).

## Schema

Table `pipeline_items`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `deployment_id` | String → Deployment | yes | | | |
| `author_instance` | String → Instance | yes | | | The member who did the work (the lead for its own PR) |
| `branch` | String | yes | | | `wt/<agent>/<card>` at stages 0–2, the lead branch at 3–4 |
| `card_ref` | String | | | | The wiki card it implements (e.g. `backlog/LED-44.md`) |
| `stage` | Integer | yes | `0` | 0–4 | 0 in worktree · 1 handed to lead · 2 lead reviewing · 3 integrated · 4 PR → main |
| `note` | String | | | | Latest status line (e.g. "tests pass", send-back notes) |
| `tone` | String | | `""` | enum: `""`, `ok`, `err`, `ask`, `merged` | Colors the note in Changes |
| `forge_pr_url` | String | | | | Set at stage 4 |
| `forge_pr_number` | Integer | | | | |
| `checks_status` | String | | | | Result of the project `checks` |
| `merged_at` | Timestamp | | | | Set on merge; the item then lists under Merged |

## Invariants

- `handoff(teammate, branch, summary)` creates or advances an item.
- Send back resets the item to stage 0 with the CEO's notes.
- Only the lead's `open_pr` moves an item to stage 4.

## Open questions

1. **One PR, many items.** The lead integrates several handoffs into one lead branch and opens one PR, but stages are per item. Options: the PR is its own item that references the integrated items, and merging it marks them all merged (prototype: "PR #128: settlement v2 groundwork" as an item at stage 4); or items stay per card, `open_pr` moves every stage-3 item of the deployment to stage 4 together, and merge / send back acts on all of them ([Pipeline and pull requests](../features/pipeline-and-prs.md)). Recommendations differ: PR-as-item (from the model) vs per-card items moved together (from the feature).
2. **Stage 2 transition.** Nothing in the ACP tools says when "lead reviewing" starts. Recommendation: derive it from the lead's first activity on the item (e.g. an `ask` to the author or a fetch of the branch), or drop stage 2.
3. **`tone` values.** Only the prototype defines them; confirm they're a UI concern derived from state rather than stored.
