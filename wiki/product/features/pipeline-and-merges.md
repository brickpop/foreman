---
type: spec
status: partial
tags: [feature, pipeline, git, forge]
---

# Pipeline and merges

How work moves from a member's worktree to `main`. Only the team **lead** asks for changes to reach `main`, with a **merge request**; only the **CEO** decides, on the branch's [review](./reviews.md): Merge (Foreman merges locally), Request changes, or Close; Create PR on the forge comes later. Nothing reaches a forge unless the CEO asks. Branch rules: [Sandboxes and git policy](../architecture/sandboxes-and-git.md). Forge calls: [Forge adapters](../architecture/forge-adapters.md).

The pipeline has two kinds of entries, both shown in the [project pipeline](./project-pipeline.md) (In flight and Merged on [Worktrees](./worktrees.md)). Built so far (2026-10-17): work items ([Work item → Phasing](../models/work-item.md#phasing)) and merge requests ([Merge request → Phasing](../models/merge-request.md#phasing)); since 2026-10-21 decided on their branch's page in [Reviews](./reviews.md).

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

Each work item carries a short **note**. Worktrees → In flight colors it by a **tone** (`ok`, `err`, `ask`, neutral) derived from the work item's state, never stored, e.g. "checks pass" (`ok`) or "2 review comments" (`ask`).

## Review and merge

Only the lead calls `request_merge(branch, entry)`, naming one of its own lead branches (`lead/<own team>/*`; anything else is an error) and a wiki entry, usually the task card, that explains what was done. Foreman runs the project `checks`, creates a **merge request** for each repo in which the branch has new commits, all in the branch's one [review](../models/review.md), and raises the review's **review** request. The merge request carries the work items whose branch head is contained in that lead branch (git ancestry) and that aren't in another merge request yet; they leave "In flight" and are listed under it. **Several merge requests can be open at once**: a lead can have several lead branches, each with at most one open merge request, and other teams' leads on the same project have their own. Calling `request_merge` again on a branch whose merge request is open refreshes it (see *When main moves*). The CEO reads the change on the review's page in Foreman ([Reviews](./reviews.md)), with git, or on the forge after Create PR. The lead can't pick the repos: a repo where the branch has new commits gets its merge request, and the CEO ticks which ones to merge.

The review request's card shows the entry's title and the repos, and opens the review, where the CEO comments and decides:

| Action | What Foreman does |
|---|---|
| **`Merge to <main>`** (primary), for the repos ticked | Merges the branch into each ticked repo's `main` on the host, as [below](#merge-to-main). Each merged merge request and its work items move to "Merged", the lead is told, the review request resolves once nothing is left open, and merged branches are deleted under the existing rule ([Branches](../architecture/sandboxes-and-git.md#branches)). If the branch doesn't merge cleanly, nothing changes: the merge request is marked "conflicts with main" and the lead is asked to merge `main` in (*When main moves*) |
| **`Create PR on <forge>`** (only when the repo has a forge) | Pushes the branch and opens a pull request on the forge, recording its URL and number; the request then shows **`Open PR ↗`** instead, and the CEO merges there. Foreman polls the PR ([Forge adapters](../architecture/forge-adapters.md#pr-state)), see below |
| **`Request changes`** | Sends the draft comments (each with its repo, file and line, or general) and an optional note to the lead in one message ([Reviews → What the lead gets](./reviews.md#what-the-lead-gets)). The review waits on the lead; its next `request_merge` on the branch raises the review again, with the new diff |
| **`Close`** | The work isn't wanted. Asks what becomes of its team and branches ([Reviews → A review's page](./reviews.md#a-reviews-page)): the team keeps working, or stops; the branches stay, or are dropped, only with a team stopped or no longer running (its agents work in them). One request: the team stops first, and when it can't, nothing is closed. Then every repo still open closes (and its forge PR, once there are PRs), and a team that keeps working has its lead told to stop work on it ([Reviews → What the lead gets](./reviews.md#what-the-lead-gets)). Its work items stay with the closed merge requests, out of In flight, until a new hand-off of one of their branches frees it. **Dropped branches**: the lead branch and the members' branches its work items carry, in each repo; each head is first kept under `refs/foreman/closed/<review>/<branch>`, recoverable by hand. A branch checked out in a worktree can't be deleted, so it moves to `main` instead (`git reset --keep`). One whose worktree holds changes not committed, or a merge not finished, stays as it is, with no backup, and the close answers which and why |

After Create PR, the forge is watched:

| On the forge | Foreman |
|---|---|
| Merged | Marks the merge request merged (`merged_at`), resolves the review request, tells the lead. The merge request and its work items move to "Merged". Foreman fetches the new `main` into the repo, where every worktree sees it |
| Closed without merge | As Close with the team working and the branches kept: marks the merge request rejected, resolves the review request, tells the lead to stop work on it. Not built: [The forge](../../backlog/git-forge.md) |
| Review comments or "changes requested" | Delivers them to the lead (typed into its session when it is idle; [Protocol](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)); the merge request stays open |

**Only the code repos have merge requests.** The [project repo](../models/project-config.md#the-project-repo) gets the lead's `commit()` straight on `main`, and the [global repo](../models/home.md#the-global-repo) gets the UI's saves, the Chief of Staff's confirmed proposals included ([Chief of Staff](./chief-of-staff.md)); both reach their forge only through Sync.

The forge is chosen per project ([Project config](../models/project-config.md)); its token lives in the CEO's [secrets](../models/secrets.md), on this machine. Supported: GitHub, Forgejo, Gitea, `none`. Without a forge, Create PR is not offered.

### When main moves

Open merge requests on the same repo race each other: when one is merged, the others may now be behind `main`, or conflict with it. **Nothing is ever rebased: the lead catches up by merging `main` in**, and Foreman never resolves conflicts itself; it tells the lead who has to:

- When a merge request is merged (by Merge to main, or seen on the forge), Foreman delivers a notice (typed into the session when it is idle) to the lead of **every other open merge request targeting `main` of the same repo**: `From Foreman: main moved (lead/atlas/integration was merged). Merge it into lead/atlas/settlement-v2 with merge("main"), resolve any conflicts and commit(), then call request_merge again.`
- If the forge reports an open PR as conflicting with `main`, or Merge to main finds a conflict, the repo shows **conflicts with main** (`--err`) on its review and the lead gets the same line.
- The lead calls `merge("main")`: Foreman merges `main` into the lead branch in the lead's worktree ([Agents and git](../architecture/sandboxes-and-git.md#agents-and-git)). Conflicts go back to the lead, who resolves the markers and calls `commit`. Then it calls `request_merge` again on the same branch. Foreman reruns the checks, pushes the branch to its PR when there is one (a plain fast-forward: the branch history is never rewritten) and refreshes the request. The merge request and its review request stay the same.

### Merge to main

Decided by the CEO (2026-10-17; a squash since 2026-10-21, replacing a merge commit). The CEO's own checkout of the repo (`<project>/<repo>/`) is never left out of step with `main`:

- **Only the commit the CEO read**: the merge names the branch's head as the review page showed it, and a branch that moved since isn't merged ([Reviews](./reviews.md)).
- **Always a squash**: one new commit on `main` with what the branch changes, by the CEO's git identity (Foreman's when git has none), its message the title of the merge request's wiki entry, then a `Co-Authored-By: <Name> <email>` line per author of the branch's commits, once each, in the order they first committed. The agents' work-in-progress commits never reach `main`; each accepted piece of work is one commit there, revertable as one. When `main` has all of it already, nothing is committed.
- Foreman builds that commit without a working tree (`git merge-tree --write-tree`, `git commit-tree` with `main` as its one parent). **When the CEO's checkout has `main` checked out**, `main` then fast-forwards there to it, so its files move with it; if the CEO has uncommitted changes it would overwrite, git refuses and nothing changes: the review request says to commit or stash, then merge again. **Otherwise** (another branch, or detached), `main` is moved, and only if it is still where it was read; the CEO's files are not touched.
- Either way, a conflict changes nothing: the merge request is marked "conflicts with main" and the lead is asked to merge `main` in.
- **The merged branch then moves to the new `main`**, however it was merged (here, on the forge, by hand): its commits are not in `main`, so a next review of it would show them again. It moves only while it is still at the commit that was merged; where it is checked out (the lead's worktree), its files follow and the lead's uncommitted changes are kept (`git reset --keep`). When it can't (the lead committed since, or an uncommitted change is in the way), it stays, and the lead is told to merge `main` into it first ([Protocol → Deliveries Foreman originates](../architecture/protocol.md#deliveries-foreman-originates)).
- **What counts as merged** (here, and for a merge seen outside Foreman): merging the branch into `main` would change nothing (`git merge-tree` gives `main`'s own tree). A squash, a rebase or a merge commit on the forge all pass; a branch whose commits are in `main` is only a special case.
