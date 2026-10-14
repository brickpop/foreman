---
type: spec
status: planned
tags: [model, pipeline, git]
---

# Work item

One agent's branch on its way to the lead, stored in `state.db`: from a member's worktree, through the lead's review, to integration into the lead branch. Work items are **informational**: a handoff never creates a [request](./request.md) and needs nothing from the CEO. They show in the project's [Changes](../features/changes.md) under "In flight". Integrated work items reach `main` inside a [pull request](./pull-request.md), the pipeline's other entry. The flow is in [Pipeline and pull requests](../features/pipeline-and-prs.md).

**Why two tables, not one with a `kind`.** Work items and pull requests share almost no fields: a work item has an author, a `wt/` branch, a board card and a stage; a PR has a forge URL and number, checks, a state and a merge time. One table would leave most columns empty for each kind and make every query filter on `kind`. Two tables with one foreign key keep the queries obvious: "In flight" is the work items with no PR plus the open PRs; "Merged" is the merged PRs with their work items.

## Schema

Table `work_items`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `deployment_id` | String → Deployment | yes | | | |
| `author_instance` | String → Instance | yes | | | The member who did the work (the lead, for work it does itself) |
| `branch` | String | yes | | | `wt/<agent>/<card>` |
| `card_ref` | String | | | | The board card it implements, a wiki entry (e.g. `backlog/LED-44.md`) |
| `stage` | Integer | yes | `0` | 0–3 | 0 in worktree · 1 handed to lead · 2 lead reviewing · 3 integrated |
| `note` | String | | | | Latest status line (e.g. "tests pass", the CEO's send-back notes) |
| `checks_status` | String | | | | Result of the project `checks` run before the handoff was accepted |
| `pr_id` | String → Pull request | | | | Set when the lead's `open_pr` includes it; cleared on send back or when its PR closes without merging |

## Invariants

- `handoff(teammate, branch, summary)` creates the work item, or advances it, to stage 1 ([Team tools](../architecture/team-tools.md)).
- Stage 2 ("lead reviewing") starts when the handoff message is delivered into the lead's session (its `delivered_at`; it is queued while the lead is busy, see [Team message](./team-message.md)). By then Foreman has already fetched the branch into the lead's sandbox ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#where-the-policy-is-enforced)). Whether a change request from the lead (an `ask` to the author) sends the work item back to stage 0 is undecided ([Protocol → Open questions](../architecture/protocol.md#open-questions)).
- Stage 3 ("integrated") is set when the branch is merged into any of the lead's `lead/<team>/*` branches in the lead's sandbox.
- `pr_id` is set only by the lead's `open_pr`, only on stage-3 work items of the same deployment that have no `pr_id` yet and whose branch head is contained in the PR's branch (git ancestry), at creation or at a later update of that PR. A work item with a `pr_id` is listed under its PR, not on its own.
- A work item is merged when its PR is merged; it has no merge time of its own.
- Send back, or its PR closing without a merge, clears `pr_id` and resets the work item to stage 0 with the note; the lead decides who reworks.
- The note's **tone** (`ok`, `err`, `ask`, neutral), which Changes colors it by, is derived from the work item's state (checks, stage), never stored.
