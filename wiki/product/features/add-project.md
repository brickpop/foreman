---
type: spec
status: partial
tags: [feature, ui, home, config]
---

# Add a project

A project is a record in Foreman's database plus its [project repo](../models/project-config.md#the-project-repo) at `~/.foreman/projects/<name>/`; its code repos live anywhere in the CEO's home folder ([Project config → The project record](../models/project-config.md#the-project-record)). **Foreman never clones code**: a code repo is one the CEO already has, or a new empty one Foreman creates at a path the CEO picks. Only a project repo can be cloned, with the CEO's own git.

## The dialog

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar and from the checklist's **A project** row ([First run](./first-run.md#the-checklist)), fresh at each opening. Two steps:

1. **New project**: the **Project name** (the rules under [Names](#names) as its hint), then the **Project repo**, a segmented New | Clone:
   - **New**: "Its wiki and project.toml, in a new git repo under Foreman's home. One left there by a removed project of this name is taken over as it is."
   - **Clone**: a field for a git URL or a local path ("Cloned with your own git: your SSH keys and credential helper. Foreman stores nothing."). The button reads `Cloning…` while it runs.

   **Create project** (Enter too) calls `POST /api/projects`; a refusal shows under the fields, git's own message for a failed clone. On success the project's Overview opens behind the dialog, with the toast `<name> created` ("Made its project repo at <path>, with project.toml and a starter wiki.", "Cloned its project repo into <path>.", or, for a [takeover](#effect), "Took over the project repo left at <path>, as it was.").
2. **Repos of `<name>`**: [picking repos](#picking-repos), as many as the CEO likes, each added at once; the line above lists those added. **Later** (none yet) or **Done** closes it. The project exists from step 1: closing the dialog keeps it, and its repos are added later in [Project settings](./project-settings.md).

## Picking repos

The same form adds a repo from the dialog and from [Project settings](./project-settings.md) (`AddRepo`), a segmented **Existing repo | New empty repo**:

- **Existing repo**: the folder browser picks a git repo (its **Pick** button), or a pasted path (**Use**). The chosen path shows with **Change**. The **Repo name** follows the folder's name, made to fit the [rules](#names) (`My Repo` → `My-Repo`), until the CEO edits it. A repo without a `main` branch also shows **Main branch**, a select of its branches (the checkout's preselected): "This repo has no main branch. Pick its default: agents start their branches from it, and merges go to it. You can change it later in Project settings." ([Project config → The project record](../models/project-config.md#the-project-record)). **Add repo**.
- **New empty repo**: the browser picks the folder it goes in (**Here**, or a pasted path), and the name (required) is its new folder: "Creates <folder>/<name> with a README in a first commit, on main." **Create repo**.

**The folder browser** (`RepoPicker`, from `GET /api/folders?path=`, behind the [access token](../architecture/security.md#access-token) like every request) shows one folder at a time: its path on top, `..` to go up, then its subfolders by name (case-insensitive), hidden ones left out, a git repo's top folder (its own `.git` folder) marked `git`. A click on a folder opens it. It starts at the **start folder** and never goes above it: the folder `foreman` was started in, its real path, or `$HOME` when that is outside `$HOME` (the CEO, 2026-10-20: running Foreman from `~/dev/aragon` offers only what is under it). A folder outside the start folder, given by path, is refused (`400 INVALID`), and so is a subfolder whose real path leaves it (a symlink out). A browser can't tell where a dropped folder is on disk, by design: hence the browser and the pasted path.

**The start folder bounds adding a repo**, whichever way: the browser, a pasted path, **Locate**, a new empty repo's folder. A path outside it is refused with the reason ("… is outside <start>, where repos are added from (the folder Foreman was started in): start Foreman from a folder that holds it"). Repos already added stay usable wherever they are.

## Names

A project's name and a repo's follow a repo's rules: letters of either case, digits, `.`, `-` and `_`, starting with a letter or digit, at most 63 characters, never hidden. Names are unique in any case: a Mac's filesystem takes `Ledger` and `ledger` for one, and elsewhere they would be two names hard to tell apart. A repo is never named `project` (in any case): that is the project repo's place in a sandbox.

## Effect

`POST /api/projects {name, clone?}` ([API](../architecture/api.md)). Without `clone`, **New**:

1. Refuses a name that breaks the rules (`400 INVALID`), or one taken, in any case, by a project (`409 EXISTS`).
   **A folder already there** (`~/.foreman/projects/<name>/`, left by a project removed with its project repo kept) is **taken over** as it is, its history and edits included, with no new commit: steps 2 to 4 are skipped and the record is made (the CEO, 2026-10-20). Only a git repo's own top folder of exactly that name is taken over; otherwise `409 EXISTS` with what to do: the name in another case ("name the project <Name> to take it over"), Clone asked for ("create the project without Clone to take it over"), or not a git repo, a symlink included, since it would put the project repo outside the home ("move it away, or pick another name"). A failed takeover never deletes that folder. The answer is `201` for a project repo made or cloned, `200` for one taken over, so the dialog can say which.
2. Creates `~/.foreman/projects/<name>/`.
3. Writes `project.toml`, the scaffold's first file (`internal/home/starter/project.toml`), with the defaults ([Project config](../models/project-config.md)): `merge_to_main = "ceo"`, an empty `checks`, the gates as a commented-out starter list (`# always_ask_ceo = ["new dependency", "touches shared staging"]`) so the CEO sees the option, and a `[forge]` table with `kind = "none"`.
4. Creates the **scaffold**: the project repo's files besides `project.toml`, from Foreman's starter, a folder laid out as in the repo and built into the binary (`internal/home/starter/`, `{project}` standing for the project's name). One definition, also used to [fill a project repo that lacks them](./wiki-reader.md#no-wiki-yet). The default is modelled on Foreman's own wiki and workflow (the CEO, 2026-10-17):
   - **`AGENTS.md`**, at the repo's root, outside the wiki bundle: the project's conventions for its agents, in one file. How to operate the wiki with the `wiki` command; the structure below (technical debt kept as one page; decisions written into the spec they change, no folder of their own); specs (`status: planned | partial | built`, open questions in the spec); one task entry per task in `backlog/` (`type: task`, `title`, `status` among `backlog` (unscheduled), `todo` (scheduled), `blocked` (with a `Blocked by` line in the body saying on what), `in-progress` (also while reviewed), `done`, and no other (the CEO, 2026-10-17: five statuses, since a board shows any other as a column of its own), `priority` among `quick`, `high`, `medium`, `low`, `assignee` as an agent id, `epic`, a work-kind tag); epics in `backlog/epics/`; the lifecycle (capture, work, finish, retire); technical debt; the queries. The lead alone writes the wiki and keeps the board true, committing with `commit()`; everyone else uses `contribute()`. Every session prompt points agents at it ([Session prompt](../architecture/session-prompt.md)).
   - **`wiki/wiki.toml`**: `types = ["spec", "task", "epic", "note"]`; `backlog/**` and `inbox/**` kept out of `wiki orphans`; `../AGENTS.md` ignored (outside the bundle); three boards, the two over tasks laned by priority (`lanes = ["quick", "high", "medium", "low"]`; `quick` is work with no uncertainty, doable right now): **Board**, the scheduled tasks (`status!=backlog`) in the columns `todo`, `blocked`, `in-progress`, `done` (the CEO, 2026-10-17: about four columns as a start; `review` went, since reviewed work is still in progress); **Backlog**, the unscheduled tasks and the scheduled ones not started yet in `backlog` and `todo`: its `where` leaves out every status of work started or ended, ours (`blocked`, `in-progress`, `done`) and the usual others an agent might use instead (`doing`, `wip`, `review`, `in-review`, `testing`, `closed`, `cancelled`, `canceled`, `wontfix`, `archived`), so a stray status never lands on it: a card moved to `todo` stays there and shows on Board too (the CEO, 2026-10-17); and **Epics**, the epics by status (the CEO, 2026-10-17: two task boards rather than six columns).
   - **`.gitignore`**, at the repo's root, with defaults that fit any project repo (the CEO, 2026-10-17): the `wiki` command's index (`.wiki/`), secrets (`.env`, `.env.local`), and operating-system and editor clutter (`.DS_Store`, `Thumbs.db`, `desktop.ini`, `*.swp`, `*~`, `.idea/`, `.vscode/`, `.obsidian/`). A project made before it gets the usual notice and `Create it`.
   - **`wiki/index.md`**, titled with the project name, linking product, backlog and technical debt, and saying "Deploy a team from this project's Settings tab."
   - **`wiki/product/index.md`** (the product map), **`wiki/backlog/index.md`** (the current goal and the queries), **`wiki/technical-debt.md`**.
   - Nothing optional: `research/`, `product/glossary.md` or another folder (`discovery/`, `issues/`…) is described in AGENTS.md and created by the lead when there is something to put in it, never as an empty placeholder that invites agents to fill it (the CEO, 2026-10-17).

   The new wiki passes `wiki check`. The wiki can't be read without `wiki.toml` and `index.md`; the rest it reads fine without. The lead creates everything else. Then runs `git init` (on `main`) and commits `project.toml` and the scaffold as the repo's first commit, "Project repo".
5. Creates the project's record: no repos, the default profile.

With `clone` (a git URL or a local path), **Clone**: the same refusals, then `git clone` into `~/.foreman/projects/<name>/` with the CEO's own git on the host (their SSH keys, agent and credential helper; Foreman stores nothing), then the record. A local folder is copied, never moved in. Hardened, since the source is typed in: only the `https`, `http`, `ssh`, `git` and `file` transports (`protocol.allow`), a source starting with `-` refused, `--` before the arguments, and no terminal to prompt on (a session of its own, `GIT_TERMINAL_PROMPT=0`), so a source that wants a password fails with git's message instead of hanging. A failure is `400 INVALID` with git's message. The clone is taken as it is: one without `project.toml` or a wiki shows the usual notices and their `Create it` ([Wiki reader → No wiki yet](./wiki-reader.md#no-wiki-yet)).

Nothing is left behind if a step fails: a folder the call made is removed whole (a taken-over one stays). Marking every existing wiki entry as seen is not built yet: it needs the [UI state](../models/ui-state.md) API.

## Code repos

A project starts with none, and deploying refuses until it has one. Each route answers with the project, or a refusal that says why (`400 INVALID`, `409 EXISTS`, `409 DEPLOYED`):

- **Add** an existing repo, `POST /api/projects/{id}/repos {name, path, main?}`: an absolute path to a git repo's top folder (its `.git` a folder), with at least one commit, inside the [start folder](#picking-repos) and outside Foreman's home, and not already in the project. Its main branch is its `main` branch when it has one; otherwise the request's `main` names one of its branches, and a missing or unknown one is refused (`400 INVALID`). `GET /api/folders/branches?path=` lists a folder's branches and its checkout's, for that pick. A repo may be in several projects.
- **New** empty repo, `POST /api/projects/{id}/repos {name, path, new: true}`: a `path` that doesn't exist yet, in a folder that does, inside the start folder and outside Foreman's home. Foreman creates it on `main`, its main branch, with a `README.md` in its first commit (so agents' branches have a `main` to start from), and removes it if a later step fails.
- **Main branch** of a repo, `PUT /api/projects/{id}/repos/{repo}/main {main}`: one of its branches, else `400 INVALID`. Allowed while a team is deployed: worktrees made before stay on their branches; new branches and merges use the new one.
- **Locate** a moved repo, `PUT /api/projects/{id}/repos/{repo} {path}`: inside the start folder, and the folder must hold the repo's root commit. Existing worktrees follow it (`git worktree repair`) on the next start.
- **Rename** a repo, `PUT /api/projects/{id}/repos/{repo}/name {name}`: refused while a team is deployed on the project (`409 DEPLOYED`), and for a repo with a problem (locate it first). The new name follows the [rules](#names) (a change of case only is refused, as a name taken in any case). Each agent's existing worktree moves to the new name (`git worktree move`), so stopped deployments keep their work; if one fails, those moved go back. The project's **open** merge requests on the repo follow the new name, in the same transaction as the record; closed ones keep the old name, their history (the CEO, 2026-10-20).
- **Remove** a repo, `DELETE /api/projects/{id}/repos/{repo}`: refused while a team is deployed on the project (`409 DEPLOYED`). Its folder and branches stay.

## Removing a project

`DELETE /api/projects/{id}` (`?deleteRepo=true` for the option), `204` on success, from **Remove this project** in [Project settings](./project-settings.md). Refused while a team is deployed on the project (`409 DEPLOYED`). In order:

1. Its agents' sandboxes (`~/.foreman/sandboxes/<id>/`) go, uncommitted work included, and each code repo prunes its worktrees' records (`git worktree prune`). The repos and their branches stay.
2. Its session logs (`~/.foreman/logs/*@<id>/`) go.
3. Its record and its **history** go, in one transaction: deployments, instances, team messages, actions, work items, merge requests, requests (the CEO, 2026-10-20). Ids are built from the name (`atlas@ledger`), so a redeploy after re-adding the name would otherwise revive the old rows: re-adding it starts clean.
4. With the option (off by default), its project repo folder goes too. Without it, the folder stays in `~/.foreman/projects/`, and a new project of the same name takes it over ([Effect](#effect)).
