---
type: spec
status: planned
tags: [feature, pipeline, git, forge]
---

# Pipeline and pull requests

How work moves from a member's worktree to `main`. Only the team **lead** proposes changes to `main`, as a pull request on the project's forge; only the **CEO** merges. Each unit of work is a [pipeline item](../models/pipeline-item.md), shown in [Changes](./changes.md) ("In flight", "Merged") and, at the last stage, as a review [request](./requests.md). Branch rules: [Sandboxes and git policy](../architecture/sandboxes-and-git.md). Forge calls: [Forge adapters](../architecture/forge-adapters.md).

## Stages

| Stage | Label | Entered when |
|---|---|---|
| 0 | in worktree | A member works on `wt/<agent>/<card>` |
| 1 | handed to lead | The member calls `handoff(lead, branch, summary)` ([ACP](../architecture/acp.md)) |
| 2 | lead reviewing | The lead starts reviewing. Requesting changes (an `ask` to the author) sends the item back to stage 0 with a note |
| 3 | integrated | Merged into `lead/integration` in the lead's sandbox; checks run |
| 4 | PR → main | The lead calls `open_pr`. Foreman pushes `lead/integration` to the forge, opens the PR, records its URL and check status, and raises a **review** request |

Each item carries a short **note** with a **tone** (`ok`, `err`, `ask`, neutral) that Changes colors, e.g. "checks pass" (`ok`) or "2 review comments" (`ask`).

## CEO actions

On a review request:

- **Merge**: Foreman merges through the forge API (or a local `git merge --no-ff` when the forge is `none`), fast-forwards the project repo's `main`, records `merged_at`, and notifies the lead. Sandboxes fetch the new `main` when next idle.
- **Send back**: the CEO writes notes. Foreman posts them on the PR (when a forge is used), writes them into the lead's session, and resets the item to stage 0. The lead decides who reworks.
- **Open session**: the lead's session in [Sessions](./sessions.md).
- **Open on forge ↗**: the PR URL in a new tab.

Chief of Staff config changes follow the same review: Merge applies branch `foreman/config-N` to `main`; Discard deletes the branch.

The forge is chosen per project ([Project config](../models/project-config.md)); its token lives in workspace [secrets](../models/secrets.md). Supported: GitHub, Forgejo, Gitea, `none`.

## Open questions

1. **Send-back target.** The prototype writes the notes into the PR author's session; PRD says the lead's. Only leads open PRs, so they coincide. Recommendation: the lead's session, by definition.
2. **Updating the CEO's `main` after a merge.** After a merge, Foreman fast-forwards the project repo's `main`, which fails if the CEO has `main` checked out with local changes or local commits, or is on another branch. Options: update the `main` ref when it isn't checked out, or fast-forward when it is and the tree is clean; otherwise (a) leave it and say so in the merge toast, or (b) raise a blocked request explaining the local repo couldn't be updated. The merge itself succeeds on the forge either way. Recommendations differ: (a) from [Sandboxes and git policy](../architecture/sandboxes-and-git.md), (b) from this entry.

Whether pipeline items are per card or per PR is owned by [Pipeline item](../models/pipeline-item.md#open-questions).
