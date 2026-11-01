---
type: task
status: todo
priority: low
tags: [feature]
epic: git
---

# Remove a stopped deployment's worktrees

**Why:** Stop keeps a deployment's worktrees, so a redeploy resumes where it left off ([Sandboxes and git policy → Branches](../product/architecture/sandboxes-and-git.md#branches)). Nothing removes them today: an idle worktree keeps its branch checked out (it can't be deleted), and its changes not committed sit where only the project's Deployment section flags them (the CEO, 2026-10-22: an explicit action, never a side effect of Stop or Close).

## What it does

- On a stopped deployment's worktrees (listed as `not on the team` in the project's Deployment section, or on the Team page), **Remove worktrees**: each of its agents' worktrees goes (`git worktree remove`), its changes not committed first saved as a commit under `refs/foreman/` like `discard()` does, so nothing is lost.
- Its branches stay; the next deploy makes fresh worktrees on the home branches ([Resume, clean desks, discard and backups](./archive/resume-and-clean-desks.md)).

**Done when:** the action is in the project's Deployment section (where to put it, decided with the CEO), refused while the deployment runs, and tested: worktrees gone, changes recoverable, branches kept, a redeploy recreates the worktrees.
