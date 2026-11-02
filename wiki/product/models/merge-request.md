---
type: spec
status: partial
tags: [model, pipeline, git, forge]
---

# Merge request

A lead's request to merge one of its branches into one of a repo's [targets](../features/pipeline-and-merges.md#merge-targets) (its default branch unless the lead names another), stored in `state.db`. It is the pipeline's own entry, separate from the [work items](./work-item.md) it carries. The lead creates it with `request_merge(branch, entry, into?)`, naming a wiki entry (a task card) that explains what was done. It groups the work items whose branch head is contained in its branch and that aren't in another merge request yet, and it belongs to its branch's [review](./review.md): every repo a branch changed is decided on one review page, where the CEO comments, **merges** the repos ticked (Foreman merges locally), **requests changes** or **dismisses** it. Create PR on the forge comes later. Nothing reaches a forge unless the CEO chooses Create PR; a pull request on a forge is then just this merge request's forge side (its `forge_pr_*` fields). The flow is in [Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge).

It applies to the project's code repos, where the lead branches are. The [project repo](./project-config.md#the-project-repo) and the [global repo](./home.md#the-global-repo) have none: the lead commits the knowledge base straight to `main`, and the [Chief of Staff](../features/chief-of-staff.md)'s team changes are proposals the CEO confirms on its page, not merge requests.

## Schema

Table `merge_requests`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `project_id` | String | yes | | | The project |
| `deployment_id` | String → Deployment | yes | | | The lead's deployment |
| `author_instance` | String → Instance | yes | | | The lead that called `request_merge` |
| `repo` | String | yes | | | The code repo it merges into |
| `target` | String | yes | its repo's default branch | one of the repo's targets | The branch it merges into: `into` of the lead's `request_merge`, else the default branch; the CEO can change it on its review while it is open. Following the repo's default when that changes, while open ([Merge targets](../features/pipeline-and-merges.md#merge-targets)) |
| `branch` | String | yes | | | A lead branch `lead/<project>/<team>/<topic>` |
| `entry` | String | yes | | a path in the project's wiki | The task card explaining what was done, from `request_merge`; its title is the merge request's title |
| `state` | String | yes | `open` | enum: `open`, `merged`, `rejected`, `withdrawn` | |
| `checks_status` | String | | | | Result of the project `checks` run on the latest `request_merge` for this branch |
| `conflicting` | Boolean | | `false` | | The branch doesn't merge cleanly into its target (found by Merge, or from the forge's mergeability); shown as "conflicts with `<target>`"; cleared when the CEO changes the target |
| `forge_pr_url` | String | | | | Set by Create PR |
| `forge_pr_number` | Integer | | | | Set by Create PR |
| `note` | String | | | | Latest status line (e.g. "2 review comments", the CEO's reason for rejecting, the reason it was withdrawn) |
| `review_id` | String → Review | yes | | | The [review](./review.md) it is decided in: its branch's open one, or a new one |
| `reviewed_head` | String | | | | The branch head at the CEO's last Request changes: where "since your last review" starts |
| `head` | String | | | | The branch head when it left `open`, kept at `refs/foreman/decided/<id>` so its decided diff stays readable ([Reviews → Decided](../features/reviews.md#decided)) |
| `base` | String | | | | The commit that branch parted from its target at (empty for a fast-forward): the decided diff is `base…head` |
| `created_at` | Timestamp | yes | now | | |
| `merged_at` | Timestamp | | | | Set when Foreman merged it (Merge) or saw it merged on the forge |
| `rejected_at` | Timestamp | | | | Set by Dismiss (on its review), or when its PR is closed on the forge without a merge |

## Invariants

- Only `request_merge(branch, entry)` creates one: from a lead, for its own `lead/<project>/<team>/*` branch. At most one `open` merge request per (repo, branch); calling `request_merge` again on that branch refreshes it (checks rerun; with a PR on the forge, Foreman pushes the new head) instead of creating another. Any number can be open per deployment and per project.
- `request_merge` creates one merge request for each repo in which merging the branch into its target would change something; `into` must be a target of every one of them, or none is created.
- It carries the work items whose branch head is an ancestor of its branch head and that have no `merge_request_id` yet; they get its id. Work items integrated into the branch after a refresh are added at that refresh.
- It joins its branch's open review (the review of any open merge request on that branch), or starts one. The **review request** is the review's, one per review ([Review → Invariants](./review.md#invariants)).
- **Foreman merges only on the CEO's Merge**, on its review, never on its own; afterwards merged branches are deleted under the existing rule ([Sandboxes and git policy → Branches](../architecture/sandboxes-and-git.md#branches)).
- **Nothing goes to a forge automatically.** Only Create PR pushes the branch and opens a PR; from then on Foreman watches it, and a merge or a close on the forge ends the merge request.
- When a merge request is merged, the lead of every other `open` merge request into the same `target` of the same `repo` is told that it moved and asked to merge it in with `merge("<target>")` ([When a target moves](../features/pipeline-and-merges.md#when-a-target-moves)). Nothing is ever rebased.
- `merged`, `rejected` and `withdrawn` are final. Withdrawn (by the lead's `withdraw_merge`, or by Foreman when a merged review of the same lead holds all of its review): the lead took it back, not the CEO; its work items are freed, their commits still on the lead branch, and go to the superseding merge request in their repo, or one of the lead's open ones whose branch holds them, or else wait free for the next `request_merge` ([Reviews → Withdrawn](../features/reviews.md#withdrawn)). Merged: its work items are merged with it, the lead is told, and both show under "Merged" in the project's [Worktrees](../features/project-team.md) tab ([Project pipeline](../features/project-pipeline.md#3-merged)). Rejected (by Dismiss on its review, or its PR closed on the forge): the work isn't wanted, the lead is told to stop work on it while its team keeps working (a team stopped with the dismissal hears nothing), its work items stay carried by it (out of In flight), and its branches stay unless the CEO dropped them, which only a team stopped or not running allows ([Pipeline and merges → Review and merge](../features/pipeline-and-merges.md#review-and-merge)).
- The note's tone is derived, as for [work items](./work-item.md#invariants).

## Phasing

Built (2026-10-17, the Git epic's `git-merge-to-main`): the table; the lead's `request_merge` (one per repo with new commits, refreshed when called again, carrying the integrated work items); Merge to main as [decided](../features/pipeline-and-merges.md#merge-to-a-target) and Reject, each telling the lead; noticing a merge made outside Foreman within 30 s (Workflow's `Reconcile`), and the "main moved" notice to the other open merge requests' leads. Since 2026-10-21 it is decided on its [review](./review.md)'s page, with Merge (the repos ticked) and Close (the old Reject; Dismiss since 2026-10-22) ([Reviews](../features/reviews.md)), instead of on its request's card. Since 2026-10-22 the lead can withdraw it, and Foreman does when a merged review holds all of its review ([Reviews → Withdrawn](../features/reviews.md#withdrawn)). Since 2026-11-02 it has its `target` ([Merge targets](../features/pipeline-and-merges.md#merge-targets)); those made before went to their repo's default branch. Not yet: the checks (`git-checks`); Create PR and everything on the forge (`git-forge`). Merged branches are pruned under the existing rule, when the deployment stops.
