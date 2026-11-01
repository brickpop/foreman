---
type: spec
status: partial
tags: [model, pipeline, git]
---

# Work item

One agent's branch on its way to the lead, stored in `state.db`: from a member's worktree, through the lead's review, to integration into the lead branch. Work items are **informational**: a handoff never creates a [request](./request.md) and needs nothing from the CEO. They show in the project's [Repositories](../features/repos.md) tab under "In flight" ([Project pipeline](../features/project-pipeline.md#2-in-flight)). Integrated work items reach `main` inside a [merge request](./merge-request.md), the pipeline's other entry. The flow is in [Pipeline and merges](../features/pipeline-and-merges.md).

**Why two tables, not one with a `kind`.** Work items and merge requests share almost no fields: a work item has an author, a `wt/` branch, a board card and a stage; a merge request has a wiki entry, checks, an optional forge PR, a state and a merge time. One table would leave most columns empty for each kind and make every query filter on `kind`. Two tables with one foreign key keep the queries obvious: "In flight" is the work items with no PR plus the open PRs; "Merged" is the merged PRs with their work items.

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
| `note` | String | | | | Latest status line (e.g. "tests pass", the CEO's reason for rejecting its merge request) |
| `checks_status` | String | | | | Result of the project `checks` run before the handoff was accepted |
| `merge_request_id` | String → Merge request | | | | Set when the lead's `request_merge` includes it; kept when its merge request is rejected, until a new hand-off of its branch |

## Invariants

- `handoff(teammate, branch, summary)` creates the work item, or advances it, to stage 1 ([Team tools](../architecture/team-tools.md)).
- Stage 2 ("lead reviewing") starts when the handoff message is delivered into the lead's session (its `delivered_at`; it is queued while the lead is busy, see [Team message](./team-message.md)). The lead sees the branch at once: every worktree shares the repo ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#hand-offs)). A change request from the lead (an `ask` to the author) moves nothing; the author's next hand-off of the branch brings it back to stage 1 ([Protocol → Work items and merge requests](../architecture/protocol.md#work-items-and-merge-requests)).
- Stage 3 ("integrated") is set when the branch is merged into any of the lead's `lead/<team>/*` branches in the lead's worktree: checked right after the lead's `merge` or `commit`, in every repo that has the branch.
- `merge_request_id` is set only by the lead's `request_merge`, only on stage-3 work items of the same deployment that have no `merge_request_id` yet and whose branch head is contained in the merge request's branch (git ancestry), at creation or at a later refresh of it. A work item with a `merge_request_id` is listed under its merge request, not on its own.
- A work item is merged when its merge request is merged; it has no merge time of its own.
- Its merge request being rejected (Close on its review, or its PR closed on the forge) leaves it carried by that merge request, at its stage: the work isn't wanted, so it leaves In flight with it. A new hand-off of its branch frees it (clears `merge_request_id`) and starts it over as new work; so does a later `request_merge` taking it, as a work item carried by a rejected merge request counts as free.
- The note's **tone** (`ok`, `err`, `ask`, neutral), which Deployment → In flight colors it by, is derived from the work item's state (checks, stage), never stored.

## Phasing

Built (2026-10-17, the Git epic's `git-work-items`): the table, stages 1–3 from hand-offs, deliveries and the lead's merges, and In flight (now in the Deployment section) from `GET /api/pipeline` with `pipeline.changed`. Not yet: `checks_status` and the "checks fail" stay at 0 ([git-checks](../../backlog/git-checks.md)); `card_ref`, which nothing fills yet (In flight shows the branch instead).
