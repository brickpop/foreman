---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# One Foreman folder: hq/, projects/ and worktrees/

**Why:** the CEO, 2026-11-04. Per-project workspaces (`<workspace>/{project,agents}`, from [Project workspaces](./project-workspaces.md)) are messy: the CEO picks a folder per project, and what to keep sits beside what can go. Wanted instead: "some sort of foreman/worktrees folder where I know that things can be safely removed... and a separate foreman/projects/... with each project's meta repo". Extended on 2026-11-08 with the [Director](./epics/director.md) design: the reserved repos become **HQs**, and the global repo's content moves out of the hidden home. First of two changes (split on 2026-11-05): this one moves the folders and renames; [The wiki at the HQ's root](./wiki-at-root.md) follows.

## The split (the CEO, 2026-11-08)

**`~/foreman/` holds what the CEO owns; `~/.foreman/` holds Foreman's machinery.**

```
~/foreman/                              the foreman_folder setting (default ~/foreman)
  hq/                                   Foreman's HQ (git)
    teams/                              team definitions (was ~/.foreman/config/teams/)
    instructions.md                     the CEO's instructions to every agent (was settings' `instructions`)
    outcomes/                           the Director's board (its card: The Director's board)
  projects/<project>/
    hq/                                 the project's HQ (git; was "the project repo"): wiki, AGENTS.md, project.toml
    files/inbox/, files/shared/         files for and from the agents (unchanged in meaning)
  worktrees/<project>/<agent>/<repo>/   the agents' worktrees: disposable

~/.foreman/
  settings.toml                         this machine's settings, a plain file (no git), preferences.toml merged in
  state.db, logs/, credentials/, profiles/, images/, access-token, server.lock
  harness/<project>/<agent>/, harness/director/   the agents' private harness folders and conversations
```

- **HQ** is the name of the reserved repo, the folder and the concept alike (the CEO, 2026-11-08): "the project's HQ" replaces "the project repo" everywhere (specs, UI, code, the session prompt). The lead's `commit()` names it `hq` (was `project`), and so does its mount in the sandbox. Foreman's HQ is the same kind of thing one level up: the Director's, as a project's HQ is its lead's.
- **`projects/` and `hq/` are what to back up and sync.** **`worktrees/` is what can go:** every commit is in the code repo, so removing a worktree loses only what an agent left uncommitted, and Foreman makes it again at the agent's next start. To make that true: a worktree folder gone is no error (Foreman prunes its git record and makes it again), and the CEO chooses what happens to uncommitted work (the CEO, 2026-11-05): Deployment's [Clean up](../product/features/project-team.md#earlier-teams) asks with a checkbox, off at each opening, `Also remove worktrees with uncommitted work`, listing them; off, they stay as today.
- **`files/`** (the CEO's inbox for the agents and their shared files, 2026-11-06) sits beside the project's HQ, not in it.
- **The global repo goes.** Its `teams/` move to Foreman's HQ; `instructions` leaves `settings.toml` for `hq/instructions.md` (prose, editable in any editor, versioned); `settings.toml` stays in `~/.foreman` as a plain file, since it is this machine's setup (port, engine, limits, notifications, forge accounts) and it names `foreman_folder`, so it can't live inside it. Its history is no longer kept (accepted). Sync pushes Foreman's HQ instead (`global_forge` → `hq_forge`). The file watcher follows `hq/teams/`.
- **`preferences.toml` merges into `settings.toml`** (the CEO, 2026-11-08): the two were apart only because settings were in git and preferences weren't. The theme and the accent become settings fields; the browser's first-frame copy stays. One settings API serves them (`/api/preferences` goes).
- **Foreman's HQ is made on first start** (`git init`, a first commit): `teams/`, an empty `instructions.md`, and the wiki scaffold at its root ([The wiki at the HQ's root](./wiki-at-root.md)).
- **`foreman_folder`**: a global setting (renaming `projects_folder`), default `~/foreman`, kept as a `~` path; Foreman derives every path from it and the project's id, no per-project workspace (its column goes). The reserved-folder check becomes this one folder. Changing it while Foreman has projects: Foreman doesn't move anything; it says so (the CEO moves the folder by hand with Foreman stopped).

## Steps

- [ ] paths: `foreman_folder`; Foreman's HQ, each project's `hq/`, `files/`, `worktrees/` derived from it; the workspace column dropped
- [ ] the global repo replaced by Foreman's HQ: teams, `instructions.md`, `settings.toml` out of git, `hq_forge` and Sync, the watcher
- [ ] `preferences.toml` merged into `settings.toml`: `theme` and `accent` fields, one API, Settings → Appearance saving through it
- [ ] the rename "project repo" → "HQ": code, UI, the `commit` tool's `hq`, the sandbox mount, the session prompt
- [ ] the New project dialog loses its Workspace field (and [Restructure the New project dialog](./new-project-dialog.md) its workspace **Change**)
- [ ] a missing worktree is made again at the agent's start (its git record pruned first)
- [ ] Clean up's `Also remove worktrees with uncommitted work` option (the request carries it; the result lists what it removed with its uncommitted files)
- [ ] the old migration goes, with what [Remove the workspace migration](./remove-workspace-migration.md) lists: the CEO's projects move with the script below
- [ ] specs: home, global-settings, ui-state (preferences), api, team-definition, project-config, add-project, project-settings, project-team (Clean up), sandboxes-and-git, session-prompt, team-tools (`commit`), forge-adapters (Sync), settings, first-run, glossary
- [ ] tests: the paths; Foreman's HQ made on first start; instructions read from `instructions.md`; theme and accent round-trip through `settings.toml`; Sync on the HQ; a deleted worktree made again; Clean up with and without the uncommitted option

## Moving the CEO's setup: a script, not shipped

The CEO, 2026-11-05 and 2026-11-08: "a temporary script so that I can migrate the projects I currently have". Written once both changes land, as `scripts/move-to-foreman-folder.sh`, committed by the CEO on a branch of its own that is never merged, so `main` has nothing to remove. Run by the CEO with Foreman stopped:

- **Foreman's HQ:** move `~/.foreman/config/` to `~/foreman/hq/` (its git history kept), then in one commit: `settings.toml` out of the repo to `~/.foreman/settings.toml` with `instructions` cut into `instructions.md` and `projects_folder` → `foreman_folder`, `global_forge` → `hq_forge`, and `~/.foreman/preferences.toml`'s `theme` and `accent` merged in (the file deleted); add the wiki scaffold. Delete `~/.foreman/cos/` (the old Chief of Staff conversation, [Chief of Staff → Director](./director-rename.md)).
- **Each project:** move its project repo to `projects/<id>/hq/` (and its wiki up to the root, one commit, [the second change](./wiki-at-root.md)), its `files/` to `projects/<id>/files/`, its worktrees to `worktrees/<id>/`; `git worktree repair` in each code repo; rename each agent's conversation folder under `~/.foreman/harness/<id>/<agent>/` (Claude Code and Pi file conversations by working folder); update the instances' paths in state.db and drop the workspace column's values (`sqlite3`, which macOS has).
- A dry run first that prints every move. Nothing to remove afterwards. Between the two changes Foreman runs on the new folders with the wiki still in `wiki/`, so the script waits for both.
