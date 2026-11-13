---
type: task
status: done
priority: medium
tags: [feature]
epic: git
---

# Branch cleanup over all of a project's repos

**Why:** the CEO, 2026-11-13: cleaning up one repo at a time was tedious, and the CEO's own merged branches were folded behind the agents' unmerged work. Now one cleanup covers every code repo, the safest to delete first ([Repos → Branch cleanup](../product/features/repos.md#branch-cleanup)).

- [x] `GET`/`POST /api/projects/:id/branch-cleanup`, the repos read side by side; each branch with its repo, each pick `{repo, branch}`; the per-repo routes and the repo page's button gone
- [x] Sections: agents' merged and superseded, then the CEO's (open, ticked), then agents' unmerged and stray worktrees (folded, unticked), then staying
- [x] Tests: across repos (same name in two repos, a pick in an unknown repo), the sections' order and first ticks
