---
type: spec
status: partial
tags: [feature, ui, home, config]
---

# Add a project

A project is a record in Foreman's database plus its [project repo](../models/project-config.md#the-project-repo) at `~/.foreman/projects/<name>/`; its code repos live anywhere in the CEO's home folder ([Project config → The project record](../models/project-config.md#the-project-record)). **Foreman never clones code**: a code repo is one the CEO already has, or a new empty one Foreman creates at a path the CEO picks.

**What exists now** is the routes below, without a dialog. The **New project** dialog, the sidebar's **`+ New project`**, the actions of the setup checklist's **A project** and **A code repo** rows ([First run](./first-run.md#the-checklist)) and the repo management in [Project settings](./project-settings.md) come back with [Add a project, anywhere](../../backlog/add-project-anywhere.md): the old dialog listed a workspace folder's subfolders, which no longer exist (the CEO, 2026-10-20).

## The dialog (planned)

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar and from the checklist's **A project** row. A name, then the project repo: **New** (a starter wiki) or **Clone** (a git URL); then the code repos, picked from disk (a folder browser of the git repos under `$HOME`, or a path), each named after its folder unless renamed, or a new empty one at a path the CEO picks. On success it opens the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast ([App shell](./app-shell.md#toasts): "Deploy a team from its Settings tab."). The details are designed with that card.

## Names

A project's name and a repo's follow a repo's rules: letters of either case, digits, `.`, `-` and `_`, starting with a letter or digit, at most 63 characters, never hidden. Names are unique in any case: a Mac's filesystem takes `Ledger` and `ledger` for one, and elsewhere they would be two names hard to tell apart. A repo is never named `project` (in any case): that is the project repo's place in a sandbox.

## Effect

`POST /api/projects {name}` (**New**, [API](../architecture/api.md)):

1. Refuses a name that breaks the rules (`400 INVALID`), or one taken, in any case, by a project or by a folder left in `~/.foreman/projects/` (`409 EXISTS`).
2. Creates `~/.foreman/projects/<name>/`.
3. Writes `project.toml` with the defaults ([Project config](../models/project-config.md)): `main = "main"`, `merge_to_main = "ceo"`, an empty `checks`, the gates as a commented-out starter list (`# always_ask_ceo = ["new dependency", "touches shared staging"]`) so the CEO sees the option, and a `[forge]` table with `kind = "none"`.
4. Creates the **scaffold**: the project repo's files besides `project.toml`, from Foreman's starter, a folder laid out as in the repo and built into the binary (`internal/home/starter/`, `{project}` standing for the project's name). One definition, also used to [fill a project repo that lacks them](./wiki-reader.md#no-wiki-yet). The default is modelled on Foreman's own wiki and workflow (the CEO, 2026-10-17):
   - **`AGENTS.md`**, at the repo's root, outside the wiki bundle: the project's conventions for its agents, in one file. How to operate the wiki with the `wiki` command; the structure below (technical debt kept as one page; decisions written into the spec they change, no folder of their own); specs (`status: planned | partial | built`, open questions in the spec); one task entry per task in `backlog/` (`type: task`, `title`, `status` among `backlog` (unscheduled), `todo` (scheduled), `blocked` (with a `Blocked by` line in the body saying on what), `in-progress` (also while reviewed), `done`, and no other (the CEO, 2026-10-17: five statuses, since a board shows any other as a column of its own), `priority` among `quick`, `high`, `medium`, `low`, `assignee` as an agent id, `epic`, a work-kind tag); epics in `backlog/epics/`; the lifecycle (capture, work, finish, retire); technical debt; the queries. The lead alone writes the wiki and keeps the board true, committing with `commit()`; everyone else uses `contribute()`. Every session prompt points agents at it ([Session prompt](../architecture/session-prompt.md)).
   - **`wiki/wiki.toml`**: `types = ["spec", "task", "epic", "note"]`; `backlog/**` and `inbox/**` kept out of `wiki orphans`; `../AGENTS.md` ignored (outside the bundle); three boards, the two over tasks laned by priority (`lanes = ["quick", "high", "medium", "low"]`; `quick` is work with no uncertainty, doable right now): **Board**, the scheduled tasks (`status!=backlog`) in the columns `todo`, `blocked`, `in-progress`, `done` (the CEO, 2026-10-17: about four columns as a start; `review` went, since reviewed work is still in progress); **Backlog**, the unscheduled tasks and the scheduled ones not started yet in `backlog` and `todo`: its `where` leaves out every status of work started or ended, ours (`blocked`, `in-progress`, `done`) and the usual others an agent might use instead (`doing`, `wip`, `review`, `in-review`, `testing`, `closed`, `cancelled`, `canceled`, `wontfix`, `archived`), so a stray status never lands on it: a card moved to `todo` stays there and shows on Board too (the CEO, 2026-10-17); and **Epics**, the epics by status (the CEO, 2026-10-17: two task boards rather than six columns).
   - **`.gitignore`**, at the repo's root, with defaults that fit any project repo (the CEO, 2026-10-17): the `wiki` command's index (`.wiki/`), secrets (`.env`, `.env.local`), and operating-system and editor clutter (`.DS_Store`, `Thumbs.db`, `desktop.ini`, `*.swp`, `*~`, `.idea/`, `.vscode/`, `.obsidian/`). A project made before it gets the usual notice and `Create it`.
   - **`wiki/index.md`**, titled with the project name, linking product, backlog and technical debt, and saying "Deploy a team from this project's Settings tab."
   - **`wiki/product/index.md`** (the product map), **`wiki/backlog/index.md`** (the current goal and the queries), **`wiki/technical-debt.md`**.
   - Nothing optional: `research/`, `product/glossary.md` or another folder (`discovery/`, `issues/`…) is described in AGENTS.md and created by the lead when there is something to put in it, never as an empty placeholder that invites agents to fill it (the CEO, 2026-10-17).

   The new wiki passes `wiki check`. The wiki can't be read without `wiki.toml` and `index.md`; the rest it reads fine without. The lead creates everything else. Then runs `git init` (on `main`) and commits `project.toml` and the scaffold as the repo's first commit, "Project repo".
5. Creates the project's record: no repos, the default profile.

Nothing is left behind if a step fails: the new folder is removed whole. Marking every existing wiki entry as seen is not built yet: it needs the [UI state](../models/ui-state.md) API.

## Code repos

A project starts with none, and deploying refuses until it has one. Each route answers with the project, or a refusal that says why (`400 INVALID`, `409 EXISTS`):

- **Add** an existing repo, `POST /api/projects/{id}/repos {name, path}`: an absolute path to a git repo's top folder (its `.git` a folder), with at least one commit, inside the CEO's real home folder and outside Foreman's home, and not already in the project. A repo may be in several projects.
- **New** empty repo, `POST /api/projects/{id}/repos {name, path, new: true}`: a `path` that doesn't exist yet, in a folder that does, inside the CEO's home folder. Foreman creates it on the project's `main` with a `README.md` in its first commit (so agents' branches have a `main` to start from), and removes it if a later step fails.
- **Locate** a moved repo, `PUT /api/projects/{id}/repos/{repo} {path}`: the folder must hold the repo's root commit. Existing worktrees follow it (`git worktree repair`) on the next start.
- **Remove** a repo, `DELETE /api/projects/{id}/repos/{repo}`: refused while a team is deployed on the project (`409 DEPLOYED`). Its folder and branches stay.

Removing a project comes with [Add a project, anywhere](../../backlog/add-project-anywhere.md).
