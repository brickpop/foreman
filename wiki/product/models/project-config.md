---
type: spec
status: planned
tags: [model, config, home, forge]
---

# Project config

A project is a **record** in Foreman's database (its name, its workspace, its code repos, its profile) plus its **project repo** at `<workspace>/project/`, whose `project.toml` holds the project's rules: the checks, the gates that always reach the CEO, and the forge where the CEO can create PRs. The UI shows it **read-only**; changes are made in the CEO's editor ([Project settings](../features/project-settings.md)); changes by the [Chief of Staff](../features/chief-of-staff.md) come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). Deployments are **not** in this file: they are runtime state ([Deployment](./deployment.md)).

## The project record

Kept in `state.db` (tables `projects` and `project_repos`, [Persistence](../architecture/persistence.md)), never in a file the lead can write (the CEO, 2026-10-20):

- **name**: the project id, a slug (`big-ledger`: [Add a project → Names](../features/add-project.md#names)), unique in any case (`Ledger` and `ledger` are one project). Used in instance ids (`agent@project`), URLs and its workspace folder. The UI shows it **readable** (the CEO, 2026-11-02): hyphens and underscores as spaces, the first letter capitalised (`big-ledger` → "Big ledger"), everywhere a person reads it (sidebar, cards, titles, toasts, notifications); paths and ids keep it as it is.
- **workspace**: the project's folder, an absolute path (below). Empty for a project made before workspaces, still in Foreman's home until moved (legacy).
- **profile**: the [profile](../architecture/session-runner.md#credentials) its agents run on (a harness and its login); empty means `default`. Set from [Project settings](../features/project-settings.md) (`PUT /api/projects/{id}/profile`); it reaches each agent at its next start.
- **archived**: put away (the CEO, 2026-10-23): out of the sidebar's projects (into its collapsed **Archived** group, [App shell](../features/app-shell.md#sidebar)) and the [Dashboard](../features/dashboard.md#projects), and refused by [Deploy](../features/deploy-team.md) (`project <id> is archived: unarchive it first`). Everything else is kept and works as for any project: its record, repos, worktrees, history, pages. Set from [Project settings](../features/project-settings.md) (`PUT /api/projects/{id}/archived`); archiving a project a team is deployed on stops those deployments first, after a confirmation (the server refuses it while one runs). Only an archived project can be [removed](../features/add-project.md#removing-a-project). On this machine only, like the rest of the record.
- **code repos**: each a **name** (unique in the project in any case, never `project`), an absolute **path** anywhere in the CEO's home folder, its **root commit** (the lowest hash among the commits with no parent), kept so that **Locate** accepts only a folder holding that commit, its **default branch** and its other **targets** (below).
- **default branch**, one per code repo (the CEO, 2026-10-21; called its main branch until 2026-11-02): the protected branch its agents start their branches from and its merges go to unless the lead names another target. No agent writes it; it changes only by the CEO's decision: Merge, a PR merged on the forge, or the CEO's own git. It is `main` when the repo has a branch of that name; otherwise the add dialog asks which branch is its default (listing its branches, the checkout's preselected, `GET /api/folders/branches`). A new empty repo is made on `main`. [Repos](../features/repos.md) changes it to any branch the repo has but an agent's (`wt/*`, `lead/*`: agents write those, so none is ever a target; adding a repo refuses them too) (`PUT /api/projects/{id}/repos/{repo}/default`); worktrees made before stay on their branches; it becomes a target if it wasn't one, the old one stays a target, and open merge requests keep theirs. One a repo no longer has (renamed) is flagged there, and its agents can't start: their start fails with the reason in their pane ([Instance](./instance.md)). Repos added before 2026-10-21 got `main`. The project repo is always on `main` (Foreman makes it).
- **targets**, one list per code repo (the CEO, 2026-11-02): the protected branches its merges can go to, its default branch flagged among them, a release branch say ([Pipeline and merges → Merge targets](../features/pipeline-and-merges.md#merge-targets)). Each a **name** (a branch of that repo, never `wt/` or `lead/`), **default** (exactly one), **made by** (the lead that made it; empty for one the CEO set as the default), **from** (the commit it was made at) and **when**. The lead makes one with `create_target`, from a commit a target holds, which records it and creates its branch; picking a default branch that wasn't a target makes it one; the CEO removes one from [Repos](../features/repos.md), refused while a merge request into it is open, its branch left in git, never the default. One whose branch was deleted outside Foreman is forgotten unless a merge request goes into it. Kept in `state.db` (`repo_targets`, the default flagged `is_default`, since migration 015), never in `project.toml`.

A repo is added by the path of a git repo's top folder: its `.git` a folder (it is mounted read-only at its own path), with at least one commit, its real path inside the CEO's home folder (`$HOME`) and outside Foreman's home ([Add a project → Picking repos](../features/add-project.md#picking-repos)), and not already in the project. Or a **new** empty repo is made at a path that doesn't exist yet, inside `$HOME` too (a README in a first commit, on `main`). A repo may be in several projects (a docs repo several projects read). Foreman never moves or deletes a code repo, nor touches its files: it writes only in its `.git` (the agents' branches and the records of their worktrees, which live in the workspace) ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).

The repos are checked when Foreman loads, on every reload and before each start, never watched. A missing folder, or one that no longer holds the root commit, is a problem on the project (`repos.<name>: …`): it blocks starting the project's agents (deploy, start, restart), not the rest, until the CEO **locates** the repo (a new path, same root commit; existing worktrees follow it with `git worktree repair`) or **removes** it. Removing a repo is refused while a team is deployed on the project, and leaves its folder and branches as they are. So is renaming one, which moves the agents' worktrees to the new name and carries the open merge requests along ([Add a project → Code repos](../features/add-project.md#code-repos)). A project whose code repos are all missing, or whose project repo is gone, has its deployments stopped after a recheck ([Deployment → Lifecycle](./deployment.md#lifecycle)). The same team may be deployed on several projects that share a repo: its branches carry the project ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#branches)).

## The workspace

A project's **workspace** is a folder of the CEO's holding the project's own files: its project repo (`project/`, always there) and its agents' sandboxes (`agents/<agent>/`, a worktree of each code repo), so the CEO can find them, look at an agent's work and run its code (the CEO, 2026-10-28). Foreman's home keeps only Foreman's own data: each agent's harness folder (its config and conversations) is `~/.foreman/harness/<project>/<agent>/`, readable by the CEO only.

- Chosen at **New project**, pre-filled `<projects folder>/<name>` (the `projects_folder` [setting](./global-settings.md), `~/workspace/foreman` by default) and editable. It and its missing parents are created with the project.
- Anywhere inside `$HOME`, never inside Foreman's home; not a file; not inside a code repo of any project (its worktrees would show in that repo); not another project's workspace, inside one or around one.
- **Removing** the project keeps it: the CEO's files. Its agents' worktrees go only when asked; an empty workspace goes ([Add a project → Removing a project](../features/add-project.md#removing-a-project)).
- **Moving a project made before workspaces** (legacy, `POST /api/projects/{id}/migrate {workspace}`, from [Project settings](../features/project-settings.md)): refused while a team is deployed on it, or when the workspace's `project/` or `agents/` is there already. Its project repo moves to `<workspace>/project`, its sandboxes (a stopped deployment's worktrees too, uncommitted work included) to `<workspace>/agents`, each `.harness/` to `~/.foreman/harness/<project>/<agent>/`; each agent's conversations are carried over to the new working folder ([Harness adapters → Moving conversations](../architecture/harness-adapters.md#moving-conversations)), and each code repo learns where its worktrees went (`git worktree repair`; without it, a prune would take them for deleted). A step that fails undoes those before it. The moves are renames, so the workspace must be on the same file system as the home. Temporary: [Remove the workspace migration](../../backlog/remove-workspace-migration.md).

## The project repo

Every project has **one reserved git repo**, `<workspace>/project/`, made by **New project** (`POST /api/projects`). It carries what the project *is* rather than its code: `project.toml` and the project's knowledge base, the [wiki bundle](../architecture/wiki-engine.md) `project/wiki/` (`wiki.toml`, the board's task cards, product specs, ADRs, and `inbox/` for [contributions](../architecture/team-tools.md#tools)). [Add a project](../features/add-project.md) creates it (`git init`, `project.toml`, a starter wiki, `AGENTS.md`), or clones an existing one from a git URL or a local path with the CEO's own git ([Add a project → Effect](../features/add-project.md#effect)). It is never a code repo: agents get no worktree of it, but one live checkout shared by the whole team, writable by the lead only ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

The project repo uses the project's `forge`, like the code repos, at the URL `forge.project_repo` names explicitly (any repo name, since `project` would clash between projects under one owner). Nothing is pushed on its own: the CEO pushes its `main` with **Sync** in [Project settings](../features/project-settings.md) ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). Foreman's own writes to it (contributions, the CEO's checkbox ticks and card moves) are committed directly, one commit per write ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

## Example

```toml
description = "Settlement and reconciliation service."
checks = "npm test"                    # run in a sandbox before handoff and on a merge request
always_ask_ceo = [                     # gates: these always raise an approval request
  "new dependency",
  "touches shared staging",
  "scope change vs an accepted ADR",
]
recursive_submodules = false           # also the submodules of submodules, in the agents' worktrees

[forge]
kind = "forgejo"                       # github | forgejo | gitea | none
url = "https://localhost:3000/ledger"  # org/owner base; repo name on the forge = the repo's name in the project
project_repo = "https://localhost:3000/ledger/ledger-project"  # the project repo on the forge: any name
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `description` | String | | | one line | Shown at the head of the project's Wiki and on its Dashboard card |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on a merge request ([Pipeline and merges](../features/pipeline-and-merges.md)) |
| `forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | `none` = no forge: a review merges locally, with no Create PR ([Forge adapters](../architecture/forge-adapters.md)). A missing `forge` table means `none`; New project writes it explicitly |
| `forge.url` | String (URL) | when `kind` ≠ `none` | | | Org/owner base URL; the repo name on the forge = the repo's name in the project |
| `forge.project_repo` | String (URL) | when `kind` ≠ `none` | | | The [project repo](#the-project-repo) on the forge, given in full: its name is free, since `project` would clash between projects under one owner. Sync pushes `main` there |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |
| `recursive_submodules` | Boolean | | `false` | | Also check out the submodules of submodules in the agents' worktrees ([Sandboxes and git policy → Submodules](../architecture/sandboxes-and-git.md#submodules)). Off by default (the CEO, 2026-10-19) |

The wiki has no field: it is always `wiki/` in the project repo. A `project` or `repos` field is reported as one Foreman doesn't know: the name and the repos are the [record](#the-project-record)'s. A `main` field (every `project.toml` before 2026-10-21 has `main = "main"`) is a problem saying each code repo has its own default branch now (Repos) and to remove the line; so is a `merge_to_main` field (every `project.toml` before 2026-11-02 has `merge_to_main = "ceo"`, the only value it ever took), saying only the CEO merges, always. Like any problem, each blocks new deployments until the line is gone.

## Invariants

- The project repo's folder name is the project id.
- Foreman never writes this file after New project made it.
- No secret ever appears in this file. The forge token lives in [Secrets](./secrets.md).

## Invalid config

A `project.toml` that fails to parse or validate doesn't hide the project: it still appears, with an error banner on the project naming the file and the field. Foreman keeps the last good version in memory ([Persistence](../architecture/persistence.md#file-watching)); new deployments, and anything else that needs the invalid value, are refused; running deployments keep running.
