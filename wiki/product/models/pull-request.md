---
type: spec
status: planned
tags: [model, pipeline, git, forge]
---

# Pull request

A lead's proposal to merge one of its lead branches (`lead/<team>/<topic>`) into `main` of one repo, stored in `state.db`. Several can be open at once: one per lead branch, across leads and teams. It is the pipeline's own entry, separate from the [work items](./work-item.md) it carries: created when the lead calls `open_pr`, it groups the work items whose branch head is contained in its lead branch and that aren't in another PR yet, and it raises the pipeline's only [request](./request.md): the **review**. On a project with a forge it mirrors a PR on the forge; without one it stands for the published lead branch the CEO merges with git. The flow is in [Pipeline and pull requests](../features/pipeline-and-prs.md#review-and-merge).

## Schema

Table `pull_requests`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | |
| `project_id` | String | yes | | | The project whose `main` it targets (needed because a Chief of Staff change has no deployment) |
| `deployment_id` | String → Deployment | | | | Absent for a Chief of Staff config change |
| `author_instance` | String → Instance | | | | The lead that called `open_pr`; absent for a Chief of Staff config change |
| `repo` | String | yes | | | The project repo whose `main` it targets |
| `title` | String | yes | | | From `open_pr(branch, title, summary)` |
| `summary` | String | | | | From `open_pr` |
| `branch` | String | yes | | | The lead branch, `lead/<team>/<topic>` (`foreman/config-N` for a Chief of Staff config change) |
| `state` | String | yes | `open` | enum: `open`, `merged`, `closed` | |
| `checks_status` | String | | | | Result of the project `checks` run on the latest `open_pr` for this branch |
| `conflicting` | Boolean | | `false` | | Set from the forge's mergeability; shown as "conflicts with main" |
| `forge_pr_url` | String | | | | Absent without a forge |
| `forge_pr_number` | Integer | | | | Absent without a forge |
| `note` | String | | | | Latest status line (e.g. "2 review comments", the CEO's send-back notes) |
| `created_at` | Timestamp | yes | now | | |
| `merged_at` | Timestamp | | | | Set when Foreman sees the merge: on the forge, or the project repo's `main` containing the branch head without one |
| `closed_at` | Timestamp | | | | Set when it is closed without merging: on the forge, by Send back, or by Discard |

## Invariants

- Only a lead's `open_pr(branch, …)` creates one, for a branch matching `lead/<its own team>/*`. At most one `open` pull request per (repo, branch); calling `open_pr` again on that branch updates it (new head fetched, checks rerun) instead of creating another. Any number can be open per deployment and per project.
- It carries the work items whose branch head is an ancestor of its branch head and that have no `pr_id` yet; they get its `pr_id`. Work items integrated into the branch after an update are added at that update.
- When a pull request is merged, the lead of every other `open` pull request on the same `repo` is told that `main` moved and asked to rebase ([When main moves](../features/pipeline-and-prs.md#when-main-moves)). Foreman never rebases.
- Creating it raises exactly one **review** request (its `pull_request_id`). The request is resolved when the pull request leaves `open`.
- `merged` and `closed` are final. Merged: its work items are merged with it, the lead is told, and both show under "Merged" in [Changes](../features/changes.md). Closed (on the forge, Send back, or Discard): its work items lose their `pr_id` and return to stage 0, the lead is told.
- Foreman never merges it ([Pipeline and pull requests](../features/pipeline-and-prs.md#review-and-merge)).
- The note's tone is derived, as for [work items](./work-item.md#invariants).
- A Chief of Staff config change is a pull request with no work items and no deployment-scoped author: its `deployment_id` and `author_instance` are absent, and its request has `source: cos`.
