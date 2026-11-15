---
type: task
status: todo
priority: medium
tags: [feature]
epic: git
---

# After a merge: clean up

**Why:** once a merge request is merged (Merge to main, or seen merged outside Foreman), what it leaves behind should go, without the CEO doing it by hand (the CEO, 2026-10-21). Today the merge request and its work items move to Merged, the review resolves and the lead is told; nothing is removed.

**Built by hand since (the CEO, 2026-11-11):** agents delete their merged and superseded branches with `prune()` (the lead its teammates' too), and the CEO any agents' branch from a repo's page ([Sandboxes and git policy → Cleaning up branches](../product/architecture/sandboxes-and-git.md#cleaning-up-branches)): one rule (never checked out, never under review, a copy kept), one primitive (`Git.DeleteBranch`). What's left here is doing it **automatically** after a merge, with that same rule: no new deletion code.

## Decided (the CEO, 2026-10-21)

- **A merged lead branch is deleted** once fully in main, unless checked out (a lead idles on no branch since 2026-11-12, so it usually isn't). A branch with anything unmerged stays.
- **Built with the squash merge (2026-10-21):** every merged lead branch, `integration` or a topic, is moved to the new main when it is still at the merged commit ([Merge to main](../product/features/pipeline-and-merges.md#merge-to-a-target)): a squash leaves its commits out of main, so it had to. What's left of this bullet is deleting the topic branches that aren't checked out. "Fully in main" is now "merging it would change nothing" (`Git.Merged`), not "its head is in main": after a squash, no branch's head is.

- **Idle desks follow the new main (2026-10-22, built; since 2026-11-12 idle worktrees are detached, on no branch):** every idle worktree at a commit main holds, with nothing uncommitted, moves to main after a merge ([Resume, clean desks, discard and backups](./archive/resume-and-clean-desks.md)).

## Open questions (the CEO, 2026-10-21: defined later)

- The members' topic branches it carried (`wt/<agent>/<topic>`): deleted at the merge, or at the deployment's end as the spec says?
- The task card named by the merge request's entry: moved to `done` by Foreman, or left to the lead?

**Done when:** (once the questions are answered) a merge leaves no merged branch behind (but one checked out, which goes when its worktree does), tests cover what is deleted and what stays (unmerged work never), and the git spec matches the code.
