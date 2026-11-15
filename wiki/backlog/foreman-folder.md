---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# One Foreman folder: hq/, projects/ and worktrees/

**Why:** the CEO, 2026-11-04. Per-project workspaces (`<workspace>/{project,agents}`, from [Project workspaces](./archive/project-workspaces.md)) are messy: the CEO picks a folder per project, and what to keep sits beside what can go. Wanted instead: "some sort of foreman/worktrees folder where I know that things can be safely removed... and a separate foreman/projects/... with each project's meta repo". Extended on 2026-11-08 with the [Director](./epics/director.md) design: the reserved repos become **HQs**, and the global repo's content moves out of the hidden home. First of two changes (split on 2026-11-05): this one moves the folders and renames; [The wiki at the HQ's root](./wiki-at-root.md) follows.

## The split (the CEO, 2026-11-08)

**`~/foreman/` holds what the CEO owns; `~/.foreman/` holds Foreman's machinery.**

```
~/foreman/                              the foreman_folder setting (default ~/foreman)
  hq/                                   Foreman's HQ (git)
    teams/                              team definitions (was ~/.foreman/config/teams/)
    instructions.md                     the CEO's instructions to every agent (was settings' `instructions`)
    projects.toml                       every project's portable record ([Portable projects](./portable-projects.md))
    outcomes/                           the Director's board (its card: The Director's board)
  projects/<project>/
    hq/                                 the project's HQ (git; was "the project repo"): wiki, AGENTS.md, project.toml
    files/inbox/, files/shared/         files for and from the agents (unchanged in meaning)
  repos/<path>/                         code repos Foreman cloned, by their remote's path (`me/ledger-api`; Portable projects)
  worktrees/<project>/<agent>/<repo>/   the agents' worktrees: disposable

~/.foreman/
  settings.toml                         this machine's settings, a plain file (no git), preferences.toml merged in
  state.db, logs/, credentials/, profiles/, images/, access-token, server.lock
  harness/<project>/<agent>/, harness/director/   the agents' private harness folders and conversations
```

- **HQ** is the name of the reserved repo, the folder and the concept alike (the CEO, 2026-11-08): "the project's HQ" replaces "the project repo" everywhere (specs, UI, code, the session prompt). The lead's `commit()` names it `hq` (was `project`), and so does its mount in the sandbox. Foreman's HQ is the same kind of thing one level up: the Director's, as a project's HQ is its lead's.
- **`projects/` and `hq/` are what to back up and sync** (automatically, through each HQ's `origin`: [Portable projects](./portable-projects.md)). **`repos/` comes back from its remotes** (only what the CEO pushed). **`worktrees/` is what can go:** every commit is in the code repo, so removing a worktree loses only what an agent left uncommitted, and Foreman makes it again at the agent's next start. To make that true: a worktree folder gone is no error (Foreman prunes its git record and makes it again), and the CEO chooses what happens to uncommitted work (the CEO, 2026-11-05): Deployment's [Clean up](../product/features/project-team.md#earlier-teams) asks with a checkbox, off at each opening, `Also remove worktrees with uncommitted work`, listing them; off, they stay as today.
- **`files/`** (the CEO's inbox for the agents and their shared files, 2026-11-06) sits beside the project's HQ, not in it.
- **The global repo goes.** Its `teams/` move to Foreman's HQ; `instructions` leaves `settings.toml` for `hq/instructions.md` (prose, editable in any editor, versioned); `settings.toml` stays in `~/.foreman` as a plain file, since it is this machine's setup (port, engine, limits, notifications) and it names `foreman_folder`, so it can't live inside it. Its history is no longer kept (accepted). Of the settings, only the instructions travel (the CEO, 2026-11-15: "the only thing I'd want from it would be the system prompt"); the rest is this machine's. The forge settings go with [Remove the forge](./archive/remove-forge.md). The file watcher follows `hq/teams/` and `hq/projects.toml`.
- **`preferences.toml` merges into `settings.toml`** (the CEO, 2026-11-08): the two were apart only because settings were in git and preferences weren't. The theme and the accent become settings fields; the browser's first-frame copy stays. One settings API serves them (`/api/preferences` goes).
- **Foreman's HQ is made on first start** (`git init`, a first commit): `teams/`, an empty `instructions.md`, and the wiki scaffold at its root ([The wiki at the HQ's root](./wiki-at-root.md)).
- **`foreman_folder`**: a setting in `~/.foreman/settings.toml` (renaming `projects_folder`), default `~/foreman`, kept as a `~` path; Foreman derives every path from it and the project's id, no per-project workspace (its column goes). The `~/workspace/foreman` default goes. **No UI for it** (the CEO, 2026-11-15): Settings → System → Projects folder goes; the CEO edits the file with Foreman stopped and moves the folder by hand. `just dev` and the tests set their own. The reserved-folder check becomes this one folder.

## Steps

- [ ] paths: `foreman_folder` (no UI); Foreman's HQ, each project's `hq/`, `files/`, `repos/`, `worktrees/` derived from it; the workspace column dropped
- [ ] the global repo replaced by Foreman's HQ: teams, `instructions.md`, `settings.toml` out of git, the watcher
- [ ] `preferences.toml` merged into `settings.toml`: `theme` and `accent` fields, one API, Settings → Appearance saving through it
- [ ] the rename "project repo" → "HQ": code, UI, the `commit` tool's `hq`, the sandbox mount, the session prompt
- [ ] the New project dialog loses its Workspace field (and [Restructure the New project dialog](./new-project-dialog.md) its workspace **Change**)
- [ ] a missing worktree is made again at the agent's start (its git record pruned first)
- [ ] Clean up's `Also remove worktrees with uncommitted work` option (the request carries it; the result lists what it removed with its uncommitted files)
- [ ] the workspace migration goes (the CEO's projects move with the script below): `home.Migrate`, `POST /api/projects/{id}/migrate`, the Project settings offer, the adapters' conversation movers (`harness.Mover`), the old layout's paths (`projectsDir`, `sandboxesDir`, `legacyHarn`, the legacy branch of `Home.layout` and `Project.HarnessDir`), the launch's removal of the old `<sandbox>/project` mount point, their tests, and both items in [Technical debt](../technical-debt.md#the-workspace-migration)
- [ ] specs: home, global-settings, ui-state (preferences), api, team-definition, project-config, add-project, project-settings, project-team (Clean up), sandboxes-and-git, session-prompt, team-tools (`commit`), harness-adapters (Moving conversations goes), settings, first-run, glossary
- [ ] tests: the paths; Foreman's HQ made on first start; instructions read from `instructions.md`; theme and accent round-trip through `settings.toml`; a deleted worktree made again; Clean up with and without the uncommitted option

## Moving the CEO's setup: a script, not shipped

The CEO, 2026-11-05 and 2026-11-08: "a temporary script so that I can migrate the projects I currently have". Written once both changes land, as `scripts/move-to-foreman-folder.sh`, committed by the CEO on a branch of its own that is never merged, so `main` has nothing to remove. Run by the CEO with Foreman stopped:

- **Foreman's HQ:** move `~/.foreman/config/` to `~/foreman/hq/` (its git history kept), then in one commit: `settings.toml` out of the repo to `~/.foreman/settings.toml` with `instructions` cut into `instructions.md` and `projects_folder` → `foreman_folder`, `global_forge` and `forges` dropped (the global repo's forge URL set as its `origin`), and `~/.foreman/preferences.toml`'s `theme` and `accent` merged in (the file deleted); add the wiki scaffold. Delete `~/.foreman/cos/` (the old Chief of Staff conversation, [Chief of Staff → Director](./director-rename.md)).
- **`projects.toml`:** written from `state.db`'s project records (display name, hidden, each repo's origin, root commit and default branch; each HQ's origin), one commit in Foreman's HQ ([Portable projects](./portable-projects.md)).
- **Each project:** `project.toml`'s `[forge]` removed, its `project_repo` set as the HQ's `origin` (one commit); move its project repo to `projects/<id>/hq/` (and its wiki up to the root, one commit, [the second change](./wiki-at-root.md)), its `files/` to `projects/<id>/files/`, its worktrees to `worktrees/<id>/`; `git worktree repair` in each code repo; rename each agent's conversation folder under `~/.foreman/harness/<id>/<agent>/` (Claude Code and Pi file conversations by working folder); update the instances' paths in state.db and drop the workspace column's values (`sqlite3`, which macOS has).
- A dry run first that prints every move. Nothing to remove afterwards. Between the two changes Foreman runs on the new folders with the wiki still in `wiki/`, so the script waits for both, and for Portable projects (it writes `projects.toml`). Run on the current machine before the CEO installs the new one: the new machine then only restores ([Portable projects → Restore](./portable-projects.md)).
