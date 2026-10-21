---
type: spec
status: partial
tags: [feature, pipeline, git, forge]
---

# Pipeline and merges

How work moves from a member's worktree to `main`. Only the team **lead** asks for changes to reach `main`, with a **merge request**; only the **CEO** decides: Merge to main (Foreman merges locally), Create PR on the forge, or Reject. Nothing reaches a forge unless the CEO asks. Branch rules: [Sandboxes and git policy](../architecture/sandboxes-and-git.md). Forge calls: [Forge adapters](../architecture/forge-adapters.md).

The pipeline has two kinds of entries, both shown in the [project pipeline](./project-pipeline.md) (In flight and Merged on [Repositories](./repositories.md)). Built so far (2026-10-17): work items ([Work item → Phasing](../models/work-item.md#phasing)) and merge requests, decided from the Dashboard or the project's Overview, under Needs you ([Merge request → Phasing](../models/merge-request.md#phasing)).

- **Work items** ([model](../models/work-item.md)): one agent's branch on its way into the lead branch. Informational only: a handoff never creates a request and needs nothing from the CEO.
- **Merge requests** ([model](../models/merge-request.md)): the lead's request to merge a lead branch into `main`, carrying every work item integrated since the last one. A merge request is the only pipeline entry that raises a [request](./requests.md): the **review**.

Approvals, questions, findings and blocked requests are independent of the pipeline: agents and Foreman raise them whenever they happen, typically long before any merge request exists ([Requests](./requests.md)).

## Work item stages

| Stage | Label | Entered when |
|---|---|---|
| 0 | in worktree | A member works on `wt/<agent>/<card>`, the branch for one board card |
| 1 | handed to lead | The member calls `handoff(lead, branch, summary)` ([Team tools](../architecture/team-tools.md)) |
| 2 | lead reviewing | The handoff is delivered into the lead's session. Requesting changes (an `ask` to the author) moves nothing; the author's next hand-off of the branch brings it back to stage 1 ([Protocol → Work items and merge requests](../architecture/protocol.md#work-items-and-merge-requests)) |
| 3 | integrated | Merged into one of the lead's `lead/<team>/*` branches in the lead's worktree; checks run |

Each work item carries a short **note**. Repositories → In flight colors it by a **tone** (`ok`, `err`, `ask`, neutral) derived from the work item's state, never stored, e.g. "checks pass" (`ok`) or "2 review comments" (`ask`).

## Review and merge

Only the lead calls `request_merge(branch, entry)`, naming one of its own lead branches (`lead/<own team>/*`; anything else is an error) and a wiki entry, usually the task card, that explains what was done. Foreman runs the project `checks`, creates a **merge request** for each repo in which the branch has new commits, and raises its **review** request. The merge request carries the work items whose branch head is contained in that lead branch (git ancestry) and that aren't in another merge request yet; they leave "In flight" and are listed under it. **Several merge requests can be open at once**: a lead can have several lead branches, each with at most one open merge request, and other teams' leads on the same project have their own. Calling `request_merge` again on a branch whose merge request is open refreshes it (see *When main moves*). The CEO reviews each one's diff in Foreman ([Project pipeline](./project-pipeline.md)), with git, or on the forge after Create PR. The lead can't pick the repos: a repo where the branch has new commits gets its merge request, and the CEO merges or rejects each one.

The review request shows the entry's title, the branch and repo, the checks, and three actions:

| Action | What Foreman does |
|---|---|
| **`Merge to main`** (primary) | Merges the branch into `main` on the host, in the repo the merge request targets, as [below](#merge-to-main). The merge request and its work items move to "Merged", the request resolves, the lead is told, and merged branches are deleted under the existing rule ([Branches](../architecture/sandboxes-and-git.md#branches)). If the branch doesn't merge cleanly, nothing changes: the merge request is marked "conflicts with main" and the lead is asked to merge `main` in (*When main moves*) |
| **`Create PR on <forge>`** (only when the repo has a forge) | Pushes the branch and opens a pull request on the forge, recording its URL and number; the request then shows **`Open PR ↗`** instead, and the CEO merges there. Foreman polls the PR ([Forge adapters](../architecture/forge-adapters.md#pr-state)), see below |
| **`Reject`** | Asks for an optional message, delivers it to the author (the lead, typed into its session when idle), closes the forge PR if there is one (posting the reason as a comment), and returns the work items to stage 0. The branch stays: the lead can `switch` back to it, rework and request again |

After Create PR, the forge is watched:

| On the forge | Foreman |
|---|---|
| Merged | Marks the merge request merged (`merged_at`), resolves the review request, tells the lead. The merge request and its work items move to "Merged". Foreman fetches the new `main` into the repo, where every worktree sees it |
| Closed without merge | Marks the merge request rejected, resolves the review request, returns its work items to stage 0, tells the lead |
| Review comments or "changes requested" | Delivers them to the lead (typed into its session when it is idle; [Protocol](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)); the merge request stays open |

**Only the code repos have merge requests.** The [project repo](../models/project-config.md#the-project-repo) gets the lead's `commit()` straight on `main`, and the [global repo](../models/home.md#the-global-repo) gets the UI's saves, the Chief of Staff's confirmed proposals included ([Chief of Staff](./chief-of-staff.md)); both reach their forge only through Sync.

The forge is chosen per project ([Project config](../models/project-config.md)); its token lives in the CEO's [secrets](../models/secrets.md), on this machine. Supported: GitHub, Forgejo, Gitea, `none`. Without a forge, Create PR is not offered.

### When main moves

Open merge requests on the same repo race each other: when one is merged, the others may now be behind `main`, or conflict with it. **Nothing is ever rebased: the lead catches up by merging `main` in**, and Foreman never resolves conflicts itself; it tells the lead who has to:

- When a merge request is merged (by Merge to main, or seen on the forge), Foreman delivers a notice (typed into the session when it is idle) to the lead of **every other open merge request targeting `main` of the same repo**: `From Foreman: main moved (lead/atlas/integration was merged). Merge it into lead/atlas/settlement-v2 with merge("main"), resolve any conflicts and commit(), then call request_merge again.`
- If the forge reports an open PR as conflicting with `main`, or Merge to main finds a conflict, the review request panel shows **conflicts with main** (`--err`) and the lead gets the same line.
- The lead calls `merge("main")`: Foreman merges `main` into the lead branch in the lead's worktree ([Agents and git](../architecture/sandboxes-and-git.md#agents-and-git)). Conflicts go back to the lead, who resolves the markers and calls `commit`. Then it calls `request_merge` again on the same branch. Foreman reruns the checks, pushes the branch to its PR when there is one (a plain fast-forward: the branch history is never rewritten) and refreshes the request. The merge request and its review request stay the same.

### Merge to main

Decided by the CEO (2026-10-17). The CEO's own checkout of the repo (`<project>/<repo>/`) is never left out of step with `main`:

- **Always a merge commit** (`--no-ff`), by the CEO's git identity, with the message `Merge <title of the merge request's wiki entry>`. Each accepted piece of work stays one unit in the history, revertable as one, and `main` contains the branch head, which is how Foreman (and anyone) sees it merged.
- **When the CEO's checkout has `main` checked out**, Foreman runs the merge there, so its files move with the branch. If the CEO has uncommitted changes the merge would overwrite, git refuses on its own and nothing changes: the review request says to commit or stash, then merge again.
- **Otherwise** (another branch, or detached), Foreman builds the merge commit without a working tree (`git merge-tree --write-tree`, `git commit-tree`, then moves `main`), so the CEO's files are not touched.
- Either way, a conflict changes nothing: the merge request is marked "conflicts with main" and the lead is asked to merge `main` in.
