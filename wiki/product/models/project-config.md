---
type: spec
status: planned
tags: [model, config, home, forge]
---

# Project config

A project is a **record** in Foreman's database (its name, its code repos, its profile) plus its **project repo** at `~/.foreman/projects/<name>/`, whose `project.toml` holds the project's rules: the checks, the gates that always reach the CEO, and the forge where the CEO can create PRs. The UI shows it **read-only**; changes are made in the CEO's editor ([Project settings](../features/project-settings.md)); changes by the [Chief of Staff](../features/chief-of-staff.md) come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). Deployments are **not** in this file: they are runtime state ([Deployment](./deployment.md)).

## The project record

Kept in `state.db` (tables `projects` and `project_repos`, [Persistence](../architecture/persistence.md)), never in a file the lead can write (the CEO, 2026-10-20):

- **name**: the project id, a slug, unique in any case (`Ledger` and `ledger` are one project). Used in instance ids (`agent@project`) and URLs.
- **profile**: the [profile](../architecture/session-runner.md#credentials) its agents run on (a harness and its login); empty means `default`. Set from [Project settings](../features/project-settings.md) (`PUT /api/projects/{id}/profile`); it reaches each agent at its next start.
- **code repos**: each a **name** (unique in the project in any case, never `project`), an absolute **path** anywhere in the CEO's home folder, and its **root commit** (the lowest hash among the commits with no parent), kept so that **Locate** accepts only a folder holding that commit.

A repo is added by the path of a git repo's top folder: its `.git` a folder (it is mounted read-only at its own path), with at least one commit, its real path inside the CEO's real home folder and outside Foreman's home, and not already in the project. Or a **new** empty repo is made at a path that doesn't exist yet (a README in a first commit, on `main`). Foreman never moves or deletes a code repo, nor touches its files: it writes only in its `.git` (the agents' branches and worktrees, which live under the home) ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).

The repos are checked when Foreman loads, on every reload and before each start, never watched. A missing folder, or one that no longer holds the root commit, is a problem on the project (`repos.<name>: …`): it blocks starting the project's agents (deploy, start, restart), not the rest, until the CEO **locates** the repo (a new path, same root commit; existing worktrees follow it with `git worktree repair`) or **removes** it. Removing a repo is refused while a team is deployed on the project, and leaves its folder and branches as they are. The same team can't be deployed on two projects that share a repo: their branches would collide in that repo's `.git` (different teams never collide: agent ids are unique).

## The project repo

Every project has **one reserved git repo**, `~/.foreman/projects/<name>/`, made by **New project** (`POST /api/projects`). It carries what the project *is* rather than its code: `project.toml` and the project's knowledge base, the [wiki bundle](../architecture/wiki-engine.md) `project/wiki/` (`wiki.toml`, the board's task cards, product specs, ADRs, and `inbox/` for [contributions](../architecture/team-tools.md#tools)). [Add a project](../features/add-project.md) creates it (`git init`, `project.toml`, a starter wiki, `AGENTS.md`); cloning an existing one comes with [Add a project, anywhere](../../backlog/add-project-anywhere.md). It is never a code repo: agents get no worktree of it, but one live checkout shared by the whole team, writable by the lead only ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

The project repo uses the project's `forge`, like the code repos, at the URL `forge.project_repo` names explicitly (any repo name, since `project` would clash between projects under one owner). Nothing is pushed on its own: the CEO pushes its `main` with **Sync** in [Project settings](../features/project-settings.md) ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). Foreman's own writes to it (contributions, the CEO's checkbox ticks and card moves) are committed directly, one commit per write ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

## Example

```toml
description = "Settlement and reconciliation service."
main = "main"                          # protected branch name
checks = "npm test"                    # run in a sandbox before handoff and on a merge request
merge_to_main = "ceo"                  # v1: always ceo
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
| `description` | String | | | one line | Shown in the project header |
| `main` | String | | `main` | branch name | Protected branch. No agent writes it; it changes only by the CEO's decision: Merge to main, a PR merged on the forge, or the CEO's own git |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on a merge request ([Pipeline and merges](../features/pipeline-and-merges.md)) |
| `merge_to_main` | String | | `ceo` | enum: `ceo` | v1: always `ceo`, who chooses Merge to main (Foreman merges on that click) or Create PR and merges on the forge; Foreman never merges on its own |
| `forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | `none` = no forge: merge requests offer Merge to main and Reject, not Create PR ([Forge adapters](../architecture/forge-adapters.md)). A missing `forge` table means `none`; New project writes it explicitly |
| `forge.url` | String (URL) | when `kind` ≠ `none` | | | Org/owner base URL; the repo name on the forge = the repo's name in the project |
| `forge.project_repo` | String (URL) | when `kind` ≠ `none` | | | The [project repo](#the-project-repo) on the forge, given in full: its name is free, since `project` would clash between projects under one owner. Sync pushes `main` there |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |
| `recursive_submodules` | Boolean | | `false` | | Also check out the submodules of submodules in the agents' worktrees ([Sandboxes and git policy → Submodules](../architecture/sandboxes-and-git.md#submodules)). Off by default (the CEO, 2026-10-19) |

The wiki has no field: it is always `wiki/` in the project repo. A `project` or `repos` field is reported as one Foreman doesn't know: the name and the repos are the [record](#the-project-record)'s.

## Invariants

- The project repo's folder name is the project id.
- Foreman never writes this file after New project made it.
- No secret ever appears in this file. The forge token lives in [Secrets](./secrets.md).

## Invalid config

A `project.toml` that fails to parse or validate doesn't hide the project: it still appears, with an error banner on the project naming the file and the field. Foreman keeps the last good version in memory ([Persistence](../architecture/persistence.md#file-watching)); new deployments, and anything else that needs the invalid value, are refused; running deployments keep running.
