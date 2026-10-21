---
type: task
status: todo
priority: medium
tags: [feature]
epic: git
---

# After a merge: clean up

**Why:** once a merge request is merged (Merge to main, or seen on the forge), what it leaves behind should go, without the CEO doing it by hand (the CEO, 2026-10-21). Today the merge request and its work items move to Merged, the review resolves and the lead is told; nothing is removed.

**Already promised, not built:** [Sandboxes and git policy → Branches](../product/architecture/sandboxes-and-git.md#branches) says that once a deployment ends Foreman deletes each agent's `wt/*` branch merged into `main` (`Git.Prune`). There is no such code: the spec states it as fact.

## Decided (the CEO, 2026-10-21)

- **A merged lead branch is deleted** once fully in main; `lead/<team>/integration`, the lead's default and checked out, is moved to the new main instead, so the next topic starts from it. A branch with anything unmerged stays.
- **Built with the squash merge (2026-10-21):** every merged lead branch, `integration` or a topic, is moved to the new main when it is still at the merged commit ([Merge to main](../product/features/pipeline-and-merges.md#merge-to-main)): a squash leaves its commits out of main, so it had to. What's left of this bullet is deleting the topic branches that aren't checked out. "Fully in main" is now "merging it would change nothing" (`Git.Merged`), not "its head is in main": after a squash, no branch's head is.

- **Idle desks follow the new main (2026-10-22, built):** every home branch with nothing main lacks and nothing uncommitted moves to main after a merge ([Resume, clean desks, discard and backups](./archive/resume-and-clean-desks.md)).

## Open questions (the CEO, 2026-10-21: defined later)

- The members' topic branches it carried (`wt/<agent>/<topic>`): deleted at the merge, or at the deployment's end as the spec says?
- The task card named by the merge request's entry: moved to `done` by Foreman, or left to the lead?
- The PR on the forge and its remote branch: closed and deleted, once [The forge](./git-forge.md) is built?


**Done when:** (once the questions are answered) a merge leaves no merged branch behind (but one checked out, which goes when its worktree does), tests cover what is deleted and what stays (unmerged work never), and the git spec matches the code.
