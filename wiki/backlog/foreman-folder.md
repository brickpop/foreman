---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# One Foreman folder: projects/ and worktrees/

**Why:** the CEO, 2026-11-04. Per-project workspaces (`<workspace>/{project,agents}`, from [Project workspaces](./project-workspaces.md)) are messy: the CEO picks a folder per project, and what to keep sits beside what can go. Wanted instead: "some sort of foreman/worktrees folder where I know that things can be safely removed... and a separate foreman/projects/... with each project's meta repo". Together with the wiki at the root of the project repo (the CEO, same day: `project/wiki/backlog` becomes `backlog`), so the project repo is the wiki bundle, plus `project.toml` and `AGENTS.md`.

**The layout** (under one folder, the `foreman_folder` setting, renaming `projects_folder`: the CEO, 2026-11-05):

```
~/foreman/
  projects/<project>/                  the project repo: wiki.toml, index.md, product/, backlog/, inbox/, AGENTS.md, project.toml
  worktrees/<project>/<agent>/<repo>/  the agents' worktrees
```

- `projects/` is what to back up and sync: each project's knowledge, its rules.
- `worktrees/` is what can go: a worktree is a branch of a code repo checked out, and every commit is in the code repo. **Removing it loses only what an agent left uncommitted**, and Foreman makes it again at the agent's next start. To make that promise true: a worktree folder gone is no error (Foreman prunes its git record and makes it again), and the CEO chooses what happens to uncommitted work (the CEO, 2026-11-05: "give the option to keep/delete uncommitted stuff"): Deployment's [Clean up](../product/features/project-team.md#earlier-teams) asks with a checkbox, off at each opening, `Also remove worktrees with uncommitted work`, listing them; off, they stay as today.
- `~/.foreman` keeps Foreman's own data as now (state.db, token, logins, the harness folders with the conversations).

**Steps:**

- [ ] the paths: Foreman derives both from the project's id, no per-project workspace (its column in the database goes); the reserved-folder check becomes the one folder
- [ ] the wiki at the project repo's root: the starter scaffold, where the wiki is read, the contribute inbox (`inbox/`), the prompt's line, `ignore = ["AGENTS.md", "README.md"]`; a project repo still keeping it in `wiki/` (a clone) gets a notice saying to move it to the root, not "Create it"
- [ ] the New project dialog loses its Workspace field
- [ ] a missing worktree is made again at the agent's start (its git record pruned first)
- [ ] Clean up's `Also remove worktrees with uncommitted work` option (the request carries it; the result lists what it removed with its uncommitted files)
- [ ] the old migration goes, with what [Remove the workspace migration](./remove-workspace-migration.md) lists: the CEO's projects move with a one-off script instead (below)
- [ ] specs: add-project, project-config (the workspace section), sandboxes-and-git, wiki-reader, session-prompt, global-settings, project-settings, project-team (Clean up)
- [ ] tests: the paths, a project repo made with the wiki at its root, a `wiki/` one getting the notice, a deleted worktree made again

**Moving the CEO's projects: a script, not shipped** (the CEO: "this is ephemeral, only on my computer"). Written once the code lands as `scripts/move-to-foreman-folder.sh`, committed by the CEO on a branch of its own that is never merged (the CEO, 2026-11-05), so `main` has nothing to remove; run by the CEO with Foreman stopped, for each project: move its project repo to `projects/<id>` and its wiki up to the root (one commit), move its worktrees to `worktrees/<id>`, `git worktree repair` in each code repo, rename each agent's conversation folder under `~/.foreman/harness/<id>/<agent>/` (Claude Code files conversations by working folder), and update the instances' paths in state.db (`sqlite3`, which macOS has). Nothing to remove afterwards.
