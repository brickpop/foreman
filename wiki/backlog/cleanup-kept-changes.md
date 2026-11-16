---
type: task
status: todo
priority: low
tags: [feature, decision]
epic: install-first-run
---

# Clean up: what its uncommitted option can't keep

**Why:** the CEO, 2026-11-16, on the limits of Clean up's `Also remove worktrees with uncommitted work` ([One Foreman folder](./foreman-folder.md), [Project team → Earlier teams](../product/features/project-team.md#earlier-teams)). With the box ticked, a worktree's changes are kept as a commit under `refs/foreman/cleaned/<time>/<agent>/<repo>` (`git.Discard`) before it goes. Three gaps ([Technical debt → Foreman's folder](../technical-debt.md#foremans-folder-known-edges)):

- **Ignored files go unkept.** What the repo's `.gitignore` names (a `.env`, build output, a local database) is deleted with the worktree, and listed nowhere.
- **A change inside a submodule keeps the worktree.** Git can't keep it in the backup commit, so the worktree stays, listed as holding changes not committed, though the box was ticked.
- **Nothing brings the changes back.** They sit in a ref for 30 days, then are pruned; restoring one is `git checkout <ref> -- <file>` by hand, in a repo the CEO has to know.

## To decide

- Ignored files: list them in the confirm dialog and the result (what will go), or move them aside (`files/` of the project, say) instead of deleting them, or leave them as git does.
- Submodules: keep each dirty submodule's changes in its own backup ref, then remove; or keep refusing, with a clearer why.
- Restore: a **Restore** beside each "Removed with their uncommitted work" line (a worktree for that agent again, on its branch, with the changes back as uncommitted), or none (the ref and the hand command are enough).

## Steps

- [ ] the CEO decides the three above
- [ ] code, tests, and the specs: [Project team](../product/features/project-team.md#earlier-teams), [Sandboxes and git policy](../product/architecture/sandboxes-and-git.md), [API](../product/architecture/api.md)
- [ ] the technical-debt item's line on Clean up removed
