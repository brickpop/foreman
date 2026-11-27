---
type: spec
status: built
tags: [model, config, home]
---

# Project config

A project is a **record** in `projects.toml`, in [Foreman's HQ](./home.md#foremans-hq) (its id, display name and code repos: what travels between machines), this machine's part of it in the database (its profile, where each code repo is), plus its **HQ** at `projects/<id>/hq/` in [Foreman's folder](./home.md#foremans-folder), whose `project.toml` holds the project's rules: the checks and the gates that always reach the CEO. The UI shows it **read-only**; changes are made in the CEO's editor ([Project settings](../features/project-settings.md)); changes by the [Chief of Staff](../features/chief-of-staff.md) come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). Deployments are **not** in this file: they are runtime state ([Deployment](./deployment.md)).

## The project record

Split in two (the CEO, 2026-11-15: [Portable projects](../../backlog/portable-projects.md)), never in a file the lead can write (the CEO, 2026-10-20):

- **`projects.toml`**, in Foreman's HQ, travels with it: each project's id, display name, hidden, its HQ's origin, and each code repo's id, display name, origin, root commit and default branch ([below](#projectstoml)).
- **`state.db`** (tables `projects` and `project_repos`, [Persistence](../architecture/persistence.md)) keeps what belongs to this machine: the project's profile, and where each code repo is. A project from another machine has no row until this machine gives it one (a profile, a repo located).

The fields:

- **name**: the project id, a slug (`big-ledger`: [Add a project → Names](../features/add-project.md#names)), unique in any case (`Ledger` and `ledger` are one project). Used in instance ids (`agent@project`), URLs and its folders (`projects/<id>/`, `worktrees/<id>/`). It never changes. The UI shows it **readable** (the CEO, 2026-11-02): hyphens and underscores as spaces, the first letter capitalised (`big-ledger` → "Big ledger"), everywhere a person reads it (the project popover, cards, titles, toasts, notifications); paths and ids keep it as it is.
- **display name** (the CEO, 2026-11-04: renaming a project): any words the CEO gives it, at most 80 characters, one line, set on create (`POST /api/projects`, [Add a project](../features/add-project.md#names)) or in [Project settings](../features/project-settings.md#configuration) (`PUT /api/projects/:id/name {name}`, allowed while a team runs); "" (the default) shows the id readable, and a name that reads as the id made readable is stored as "", on create too. When set, it is what a person reads in place of the readable id (`projectName` in the UI); the id stays, so instance ids, branches (`wt/<id>/…`), folders and URLs are untouched. Two projects never read the same, in any case (the CEO, 2026-11-04): a project is refused (`409 EXISTS`) when it would show as another already shows (its display name, else its id readable, in any case); Project settings says so before Save.
- **profile** (this machine's): the [profile](../architecture/session-runner.md#credentials) its agents run on (a harness and its login); empty means `default`. Set from [Project settings](../features/project-settings.md) (`PUT /api/projects/{id}/profile`); it reaches each agent at its next start.
- **hidden** (the CEO, 2026-11-13, replacing **archived** of 2026-10-23, which also refused deploys and had to come before a removal): out of sight only, out of the popover's projects (into its collapsed **More** group, [App shell](../features/app-shell.md#the-project-popover)) and the [Dashboard](../features/dashboard.md#projects). Everything else works as for any project, deploying and a team on it included, and removing it ([Add a project → Removing a project](../features/add-project.md#removing-a-project)). Set from [Project settings](../features/project-settings.md) (`PUT /api/projects/{id}/hidden`). In `projects.toml`, so on every machine.
- **code repos**: each an **id**, made from its name on the server when it is added and on every rename (`slug`: `Ledger API` → `ledger-api`; unique in the project in any case, never `hq`, the HQ's id; repos added before keep theirs, `Ledger_API`, until renamed), what agents see and what names its worktrees, branches' paths and merge requests; its **display name** (the CEO, 2026-11-19), any words, one line, at most 80 characters, the same rule as a project's, for people only: "" shows the id readable, and a name that reads as the id made readable is stored as "" ([Add a project → Names](../features/add-project.md#names)); two repos of a project never show the same, in any case; its **remote** (its `origin` when it was added, without the credentials an https URL may hold; "" when it had none), its **root commit** (the lowest hash among the commits with no parent), kept so that **Locate** accepts only a folder holding that commit, its **default branch** (below), and, this machine's, an absolute **path** anywhere in the CEO's home folder. A repo with no path on this machine (its record came from another) is a problem, `repos.<id>: not on this machine yet`, until it is located, unless it is [a rename made on another machine](#a-rename-made-on-another-machine).
- **default branch**, one per code repo (the CEO, 2026-10-21; called its main branch until 2026-11-02): the branch its agents start their branches from, its idle desks follow, and its merges go to unless the lead names another branch ([Pipeline and merges → Where a merge goes](../features/pipeline-and-merges.md#where-a-merge-goes)). One of the CEO's branches, like every branch but an agent's: no agent writes it; it changes only by the CEO's decision: Merge, or the CEO's own git (a PR the CEO merged on a forge included). It is `main` when the repo has a branch of that name; otherwise the add dialog asks which branch is its default (listing its branches, the checkout's preselected, `GET /api/folders/branches`). A new empty repo is made on `main`. [Repos](../features/repos.md) changes it to any branch the repo has but an agent's (`wt/*`, `lead/*`: agents write those, so no merge ever goes into one; adding a repo refuses them too) (`PUT /api/projects/{id}/repos/{repo}/default`); that just sets it: worktrees made before stay on their branches, and open merge requests keep the branch they go into. One a repo no longer has (renamed) is flagged there, and its agents can't start: their start fails with the reason in their pane ([Instance](./instance.md)). Repos added before 2026-10-21 got `main`. Kept in `projects.toml` (the list of targets of 2026-11-02 went on 2026-11-06). The project's HQ is always on `main` (Foreman makes it).

A repo is added by the path of a git repo's top folder: its `.git` a folder (it is mounted read-only at its own path), with at least one commit, its real path inside the CEO's home folder (`$HOME`) and outside Foreman's home ([Add a project → Picking repos](../features/add-project.md#picking-repos)), and not already in the project. Or a **new** empty repo is made at a path that doesn't exist yet, inside `$HOME` too (a README in a first commit, on `main`). A repo may be in several projects (a docs repo several projects read). Foreman never moves or deletes a code repo, nor touches its files: it writes only in its `.git` (the agents' branches and the records of their worktrees, which live in `worktrees/<id>/`) ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).

The repos are checked when Foreman loads, on every reload and before each start, never watched. A missing folder, or one that no longer holds the root commit, is a problem on the project (`repos.<id>: …`): it blocks starting the project's agents (deploy, start, restart), not the rest, until the CEO **locates** the repo (a new path, same root commit; each existing worktree is relinked to it, `git worktree repair`, when its agent next opens the repo) or **removes** it. Removing a repo is refused while a team is deployed on the project, and leaves its folder and branches as they are. So is renaming one, which, when its id changes, moves the agents' worktrees to the new id and carries the open merge requests along ([Add a project → Code repos](../features/add-project.md#code-repos)). A project whose code repos are all missing, or whose HQ is gone, has its deployments stopped after a recheck ([Deployment → Lifecycle](./deployment.md#lifecycle)). The same team may be deployed on several projects that share a repo: its branches carry the project ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#branches)).

## projects.toml

Every project's record, by id, in [Foreman's HQ](./home.md#foremans-hq):

```toml
[ledger]
display_name = "The ledger"               # left out: the id readable
hidden = true                              # left out: false
hq = "git@github.com:me/ledger-hq.git"    # its HQ's origin; left out when it has none

[ledger.repos.ledger-api]
display_name = "Ledger API"                    # left out: the id readable
remote = "git@github.com:me/ledger-api.git"   # left out when it had none
root_commit = "a1b2c3…"                        # a full hash (40 hex, or 64 for SHA-256)
default_branch = "main"
```

- **Foreman writes it** on every change to a project (added, removed, renamed, hidden, a repo added, renamed or removed, its default changed), one commit each to Foreman's HQ, as the CEO (`Projects: …`). Locating a repo or setting a profile changes only this machine's part, not the file.
- **The CEO may edit it** by hand; it is read at start and on every change (watched). What Foreman can't use is a problem: a field it doesn't know, an id that isn't a [project name](../features/add-project.md#names), two ids or two repo ids that differ only in case, a repo id `hq` or not a folder name, a display name (a project's or a repo's) over 80 characters or of several lines, a root commit that isn't a full hash, a missing default branch or an agent's. While it has any, the projects stay as last read, every change to a project is refused (`400`, "fix it first"; Foreman would write over the CEO's edit), and the [Dashboard](../features/dashboard.md) shows the problems (`GET /api/home`, `projectsProblems`).
- **An entry taken out by hand** takes the project out of Foreman; its folders and this machine's row stay. A new project under that id later starts with a clean history: the old row goes first.
- No secret: an https remote's user and password are dropped before it is written.

### A rename made on another machine

`projects.toml` travels between machines; this machine's rows (each repo's path, the open merge requests) and the agents' worktrees don't. So a repo renamed elsewhere comes here as a new id with no path, while its row and worktrees stay under the old id. **Foreman follows it on load**: on every scan, for each repo with no path here, it looks for a row of this machine for the same project whose id the record no longer has and whose folder holds the repo's root commit. If exactly one does, it moves this machine's part as a rename here would, without writing `projects.toml`: each agent's worktree to the new id, then the row, with the open merge requests. Several renames there (`api` → `api-v2` → `ledger-api`) are one here. A rename of case only (an older `Ledger` → `ledger`) finds its row under the old case, and is followed the same way.

- **While a team is deployed on the project here**, nothing moves: its agents work in the old worktrees. The repo shows "Renamed from <old-id> on another machine: stop the team here, and Foreman follows." as its problem, and can't be used until then. Locating it meanwhile (Locate…, or Clone it) is refused with the same text (`409 DEPLOYED`): located, to its own folder or another clone, the rename would never be followed. Stopping the team rescans the project, so it follows at once. A team running on another machine is out of Foreman's sight: its agents lose the repo until it stops (their worktrees and uncommitted work stay), and the next deployment has it under its new id.
- **Two rows holding it** (two clones of the repo), or a worktree already at the new id: nothing moves. The repo shows "not on this machine yet: Locate it", or "<path> is in the way: move it, and Foreman follows.". Locating repairs the path; the old worktrees then stay, for the branch cleanup.

## The project's folder

Every path of a project follows from [Foreman's folder](./home.md#foremans-folder) and its id; the CEO picks none (the CEO, 2026-11-08, replacing a folder the CEO picked per project, 2026-10-28):

- `projects/<id>/hq/`: its [HQ](#the-projects-hq), what the project *is*.
- `projects/<id>/files/`: its **files** for the agents, whatever team is deployed: the CEO's in `files/inbox/`, read-only for the agents; the agents' own in `files/shared/`, which every agent writes (the CEO, 2026-11-06; [Sandboxes and git policy → Layout](../architecture/sandboxes-and-git.md#layout)). Both always exist.
- `worktrees/<id>/<agent>/`: each agent's sandbox, a worktree of each code repo it opened at `<sandbox>/<repo>`. Disposable: a worktree deleted by hand is made again when its agent opens that repo.

Foreman's home keeps only Foreman's own data: each agent's harness folder (its config and conversations) is `~/.foreman/harness/<id>/<agent>/`, readable by the CEO only. [Project settings](../features/project-settings.md) shows the HQ's and the worktrees' paths.

**Removing** the project (`DELETE /api/projects/{id}`) removes its record and history. Its HQ goes only when asked (`deleteHQ=true`), and so do its agents' worktrees (`deleteWorktrees=true`); `files/` stays, the CEO's; `projects/<id>/` and `worktrees/<id>/` go when left empty ([Add a project → Removing a project](../features/add-project.md#removing-a-project)). An HQ kept is taken over by a new project of the same name.

## The project's HQ

Every project has **one reserved git repo**, its **HQ** (the CEO, 2026-11-08: the repo, the folder and the concept), `projects/<id>/hq/`, made by **New project** (`POST /api/projects {name, clone?}`). It carries what the project *is* rather than its code: `project.toml` and the project's knowledge base, the [wiki bundle](../architecture/wiki-engine.md) `hq/wiki/` (`wiki.toml`, the board's task cards, specs, ADRs). [Add a project](../features/add-project.md) creates it (`git init`, `project.toml`, a starter wiki, `AGENTS.md`, one commit "Project HQ"), clones an existing one from `clone` (a git URL or a local path) with the CEO's own git, or takes over as it is an HQ already there (a removed project's, kept) ([Add a project → Effect](../features/add-project.md#effect)). In the API's repo lists it is the repo with `hq: true`; the lead's `commit` names it `hq`. It is never a code repo: agents get no worktree of it, but one live checkout shared by the whole team, writable by the lead only ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

Nothing is pushed on its own: the CEO pushes its `main` with their own git, to its `origin` (its automatic sync comes with [Portable projects](../../backlog/portable-projects.md)). Foreman's own writes to it (the CEO's checkbox ticks and card moves) are committed directly, one commit per write ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

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
landing_prefixes = ["f/", "feat/", "fix/"]  # the names a lead may propose for a branch its merge lands on
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `description` | String | | | one line | Shown at the head of the project's Wiki and on its Dashboard card |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on a merge request ([Pipeline and merges](../features/pipeline-and-merges.md)) |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |
| `recursive_submodules` | Boolean | | `false` | | Also check out the submodules of submodules in the agents' worktrees ([Sandboxes and git policy → Submodules](../architecture/sandboxes-and-git.md#submodules)). Off by default (the CEO, 2026-10-19) |
| `landing_prefixes` | Array of strings | | `["f/", "feat/", "fix/"]` | each a folder of branches, ending in `/`, never `wt/` or `lead/` | The names a lead may propose, with `request_merge`'s `land`, for a new branch its merge lands on, made only when the CEO merges ([Pipeline and merges → Where a merge goes](../features/pipeline-and-merges.md#where-a-merge-goes)). `[]`: none; the CEO names one at Merge anyway. The default when the key is left out (the CEO, 2026-11-11) |

The wiki has no field: it is the project's HQ itself, at its root (the CEO, 2026-11-04; `wiki/` in the HQ until 2026-11-16: the move script moves the CEO's, and Foreman reads only the root). A `project`, `repos` or `forge` field (the forge was removed on 2026-11-15) is reported as one Foreman doesn't know: the name and the repos are the [record](#the-project-record)'s. A `main` field (every `project.toml` before 2026-10-21 has `main = "main"`) is a problem saying each code repo has its own default branch now (Repos) and to remove the line; so is a `merge_to_main` field (every `project.toml` before 2026-11-02 has `merge_to_main = "ceo"`, the only value it ever took), saying only the CEO merges, always. Like any problem, each blocks new deployments until the line is gone.

## Invariants

- The project's folders are named by its id: `projects/<id>/`, `worktrees/<id>/`.
- Foreman never writes this file after New project made it.
- No secret ever appears in this file.

## Invalid config

A `project.toml` that fails to parse or validate doesn't hide the project: it still appears, with an error banner on the project naming the file and the field. Foreman keeps the last good version in memory ([Persistence](../architecture/persistence.md#file-watching)); new deployments, and anything else that needs the invalid value, are refused; running deployments keep running.
