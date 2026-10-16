---
type: spec
status: planned
tags: [model, pipeline, git, forge]
---

# Merge request

A lead's request to merge one of its branches into `main` of one repo, stored in `state.db`. It is the pipeline's own entry, separate from the [work items](./work-item.md) it carries. The lead creates it with `request_merge(branch, entry)`, naming a wiki entry (a task card) that explains what was done. It groups the work items whose branch head is contained in its branch and that aren't in another merge request yet, and it raises the pipeline's only [request](./request.md): the **review**, with three actions for the CEO: **Merge to main** (Foreman merges locally), **Create PR on the forge** (only when the repo has one), **Reject** with a reason. Nothing reaches a forge unless the CEO chooses Create PR; a pull request on a forge is then just this merge request's forge side (its `forge_pr_*` fields). The flow is in [Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge).

It applies to every repo Foreman knows: the project's code repos, the [project repo](./project-config.md#the-project-repo) and the [workspace repo](./workspace.md#the-workspace-repo). The [Chief of Staff](../features/chief-of-staff.md)'s changes to team files and `project.toml` are merge requests too.

## Schema

Table `merge_requests`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `project_id` | String | | | | The project; absent for a Chief of Staff change to the workspace repo |
| `deployment_id` | String → Deployment | | | | Absent for a Chief of Staff change |
| `author_instance` | String → Instance | yes | | | The lead (or the Chief of Staff) that called `request_merge` |
| `repo` | String | yes | | | The repo whose `main` it targets: a code repo, `project` (the project repo) or `workspace` (the workspace repo) |
| `branch` | String | yes | | | A lead branch `lead/<team>/<topic>`, or the Chief of Staff's `foreman/config-N` |
| `entry` | String | for a lead | | a path in the project's wiki | The task card explaining what was done, from `request_merge`; its title is the merge request's title. Decided: required for a lead; optional for the Chief of Staff, which describes its change in the request itself (its workspace-repo changes have no project wiki) |
| `state` | String | yes | `open` | enum: `open`, `merged`, `rejected` | |
| `checks_status` | String | | | | Result of the project `checks` run on the latest `request_merge` for this branch |
| `conflicting` | Boolean | | `false` | | The branch doesn't merge cleanly into `main` (found by Merge to main, or from the forge's mergeability); shown as "conflicts with main" |
| `forge_pr_url` | String | | | | Set by Create PR |
| `forge_pr_number` | Integer | | | | Set by Create PR |
| `note` | String | | | | Latest status line (e.g. "2 review comments", the CEO's reason for rejecting) |
| `created_at` | Timestamp | yes | now | | |
| `merged_at` | Timestamp | | | | Set when Foreman merged it (Merge to main) or saw it merged on the forge |
| `rejected_at` | Timestamp | | | | Set by Reject, or when its PR is closed on the forge without a merge |

## Invariants

- Only `request_merge(branch, entry)` creates one: from a lead, for its own `lead/<team>/*` branch; from the Chief of Staff, for its own config branch. At most one `open` merge request per (repo, branch); calling `request_merge` again on that branch refreshes it (checks rerun; with a PR on the forge, Foreman pushes the new head) instead of creating another. Any number can be open per deployment and per project.
- `request_merge` creates one merge request for each repo in which the branch has commits beyond `main`.
- It carries the work items whose branch head is an ancestor of its branch head and that have no `merge_request_id` yet; they get its id. Work items integrated into the branch after a refresh are added at that refresh.
- Creating it raises exactly one **review** request (its `merge_request_id`). The request is resolved when the merge request leaves `open`.
- **Foreman merges only on the CEO's Merge to main**, never on its own; afterwards merged branches are deleted under the existing rule ([Sandboxes and git policy → Branches](../architecture/sandboxes-and-git.md#branches)).
- **Nothing goes to a forge automatically.** Only Create PR pushes the branch and opens a PR; from then on Foreman watches it, and a merge or a close on the forge ends the merge request.
- When a merge request is merged, the lead of every other `open` merge request on the same `repo` is told that `main` moved and asked to merge it in with `merge("main")` ([When main moves](../features/pipeline-and-merges.md#when-main-moves)). Nothing is ever rebased.
- `merged` and `rejected` are final. Merged: its work items are merged with it, the lead is told, and both show under "Merged" in [Changes](../features/changes.md). Rejected (by Reject, or its PR closed on the forge): the reason is delivered to the lead, its work items lose their `merge_request_id` and return to stage 0, and the branch stays, so the lead can `switch` back to it and request again.
- The note's tone is derived, as for [work items](./work-item.md#invariants).
- A Chief of Staff change has no work items and no deployment; its request has `source: cos`.
