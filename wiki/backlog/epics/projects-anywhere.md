---
type: epic
status: todo
order: 3
---

# Projects anywhere

**Goal:** a project is a chosen set of repos, wherever they are on disk, not a folder of a workspace folder. One Foreman per machine; teams are the same everywhere, projects belong to the machine. Ahead of the CEO's first Foundry projects, so they are set up once (the CEO, 2026-10-20).

**Why:** today a project must be a direct subfolder of the workspace and its repos direct subfolders of the project ([Workspace](../../product/models/home.md)). So a single repo can't be a project with its own wiki, a repo can't be in two projects (`aragon/app` alone, and the `aragon` set), the CEO's `~/dev/{aragon,brickpop,agentic}` forces one project per parent folder, and two Foremans on nested folders (`~/dev` and `~/dev/brickpop`) overlap with nothing to stop them.

## Decided (the CEO, 2026-10-20)

- **One Foreman per machine, at `~/.foreman/`**, held by an OS file lock (`~/.foreman/server.lock`, released by the system even after a crash, so never stale): a second `foreman` says where the first one runs and opens it. No workspace folder argument any more.
- **What's committed and what isn't.** `~/.foreman/config/` is a git repo (the global repo) for what should be the same on every machine: **teams** and the machine's **settings**. What belongs to this machine stays out of git: **the projects and their repo paths** (in the state database: projects may be short-lived, and differ between machines), the runtime state, logs, secrets, logins (profiles), image builds, sandboxes, project repos.
- **Per-machine data lives in the home:** profiles, preferences and image builds are in `~/.foreman/` (they were in `~/.local/share/foreman`, never released, so nothing is moved), so there is one place.
- **A project is a record:** a name, its repos (paths anywhere, each with a name unique in the project), and its project repo.
- **Project repos live in `~/.foreman/projects/<name>/`**: one rule, nothing written into the CEO's code folders. A symlink from anywhere works for browsing. Their remote is managed as today (`forge.project_repo` and **Sync** in Project settings), and so is the global repo's (Settings).
- **Sandboxes live in `~/.foreman/sandboxes/<project>/<agent>/<repo>`**, not in the CEO's folders: no more `.sandboxes/` to exclude. The CEO sees an agent's work in its sandbox folder at any time, and in their own checkout as soon as it commits (worktrees share the repo's `.git`).
- **No workspaces for now.** With teams, logins and settings machine-wide, a workspace would only group projects; a group field can come later without moving anything.
- **`project.toml` keeps the project's own rules** (main branch, checks, gates, forge, `recursive_submodules`), which travel with its wiki. Paths, which mean nothing on another machine, are in the database.

## The new layout

| Where | What | In git |
|---|---|---|
| `~/.foreman/config/` (`teams/`, `settings.toml`) | teams; machine-wide settings | yes (the global repo) |
| `~/.foreman/state.db` | projects, their repos and paths; deployments, messages, requests… | no |
| `~/.foreman/projects/<name>/` | each project repo: `project.toml`, `wiki/` | its own repo |
| `~/.foreman/sandboxes/<project>/<agent>/` | agents' worktrees, the project repo mounted inside | no |
| `~/.foreman/profiles/`, `images/`, `preferences.toml`, `secrets`, `logs/`, `cos/` | logins, builds, UI preferences, forge tokens, session logs, the Chief of Staff | no |
| anywhere (`~/dev/aragon/app`) | the CEO's code repos, untouched except for branches in their `.git` | theirs |

## On a new machine, or when a repo moves

- **A new machine:** clone the global repo to `~/.foreman` (or start empty): teams and settings are there, no projects. Add each project again: clone its project repo from its forge URL, point it at the local repos. Nothing to reconcile, since paths were never shared.
- **A repo moved or deleted:** the project shows it **missing** (and its agents can't start), with **Locate…** (pick its new folder: checked to be the same repo, by a commit both have) and **Remove from project**. A project repo that's gone is the same. A project whose code repos all vanished, or whose project repo is gone, undeploys after a recheck ([Add a project, anywhere](../archive/add-project-anywhere.md); the old *Stop a deployment whose project folder was deleted* card folded into it).

## Decided too (the CEO, 2026-10-20)

- **Settings split.** Machine-wide in `settings.toml` (committed): engine, `max_sessions` (the machine's cap), stuck timeout, log cap, notifications, forge accounts' hosts. Per project, in the database: its profile (today `project_profiles.<project>` in settings, which can't name machine-local projects in a committed file).
- **A project repo comes in New** (Foreman creates it, starter wiki) **or Cloned** (from a git URL, any, a local path included). A local folder is never moved in.
- **Repo folders are picked with a folder browser served by Foreman** (listing git repos) **or a pasted path.** A browser can't tell where a dropped folder is on disk, by design. The browser is behind the access token like every request ([Security](../../product/architecture/security.md)).
- **Repos must be inside `$HOME`** (their real path, symlinks resolved), on every OS: anything else is refused when added, with the reason. On macOS that is also what the Podman machine shares by default; one rule everywhere keeps it predictable. Adding one is narrower still: only from inside the folder Foreman was started in ([Add a project, anywhere](../archive/add-project-anywhere.md)); repos already added stay usable.
- **A fresh start:** the current workspaces aren't imported. Their deployments, messages and requests stay behind.
- **Removing a project:** refused while a team is deployed; after a confirmation naming what goes, its record and its sandboxes go; **deleting its project repo folder is an option** of that confirmation, off by default (a project repo never pushed has no other copy). Code repos keep their branches. Its history (deployments, messages, requests, merge requests, session logs) is deleted with it, so re-adding the name starts clean (the CEO, 2026-10-20: ids are built from the name, so keeping the history would revive it).

## Cards

- [The home and the lock](../home-and-lock.md): `~/.foreman`, the lock, the CLI, per-machine data in it, the global repo and its `.gitignore`.
- [Projects as records, sandboxes under the home](../archive/projects-as-records.md): projects and repos in the database, project repos under `~/.foreman/projects/`, missing repos, the API, worktrees and mounts under the home, the prompt's paths.
- [Add a project, anywhere](../archive/add-project-anywhere.md): the dialog (New or Clone, then pick repos), Project settings' repos (add, rename, locate, remove), removing a project.
- [Settings by topic](../settings-by-topic.md): the Settings tabs split by topic, now that both of today's are machine-wide.
- [Home becomes Dashboard](../dashboard-rename.md): the Home screen renamed, so "home" means only `~/.foreman`.

**Specs it changes:** Workspace (rewritten as the home), Project config, Workspace settings, Persistence, Sandboxes and git policy (mounts), Session runner (mounts, credentials' folder), Session prompt (`{path}`), Add a project, Project settings, Settings, First run, App shell, API, Glossary, Security, Chief of Staff (its mounts).

**Side effects to carry:** the [container writes](../container-writes-outside-sandbox.md) card loses its cause in the prompt (`({path})` named the project folder, of which only the sandbox was mounted): only the sandbox's paths remain to show. `--user-data` (tests) became `--home`; `just dev` has its own home, `.dev/<harness>`, with the CEO's profiles and images linked in.

**Blocked by:** nothing; runs beside [Git](./git.md).

**Exit criteria:** one `foreman`, with no folder argument, runs projects made of repos from different folders, a one-repo project with its own wiki, and two projects sharing a repo, each with a team deployed at once; a second `foreman` is refused; a moved repo is located again; tests on the fake harness.
