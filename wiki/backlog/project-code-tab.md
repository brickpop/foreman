---
type: task
status: todo
priority: medium
tags: [feature, decision]
epic: git
---

# Project → Code (and Git): repos, branches, files

**Why:** the CEO, 2026-10-16. A project's page doesn't show what it holds: its code repos, the agents' branches and worktrees, the files. The CEO expects to open a project and see that, simply.

**The CEO's direction (2026-10-16):** a **Code** tab, read-only. Possibly also a **Git** tab with the history and branches. Needs refinement.

**Starting point** (proposed):
- **Code:** each code repo with its branches: `main`, and each agent's branch, saying whose it is, how many commits it's ahead of `main`, and whether its worktree has uncommitted changes. Clicking a branch browses its files, read-only. Diffs stay in Changes.
- **Git** (separate, or a section of Code): the history of `main` and of a chosen branch (commit, author, date, message), and the merges.

**To decide when the card starts:** one tab or two; whether the project repo (`project/`, the wiki) shows too; file view only, or rendered Markdown; and how much is live (the worktrees change under the CEO's eyes).

- [ ] decide the shape with the CEO, then spec, API (Git block reads: branches, log, tree, file at a ref) and UI
