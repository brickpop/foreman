---
type: spec
status: planned
tags: [model, config, workspace, forge]
---

# Project config

`<project>/project/project.toml`: what a project is, which code repos agents work on, the checks, the gates that always reach the CEO, and the forge where the CEO can create PRs. The UI shows it **read-only**; changes are made in the CEO's editor ([Project settings](../features/project-settings.md)); changes by the [Chief of Staff](../features/chief-of-staff.md) come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). Deployments are **not** in this file: they are runtime state ([Deployment](./deployment.md)).

## The project repo

Every project folder holds **one reserved git repo, `project/`**, next to its code repos. It carries what the project *is* rather than its code: `project.toml` and the project's knowledge base, the [wiki bundle](../architecture/wiki-engine.md) `project/wiki/` (`wiki.toml`, the board's task cards, product specs, ADRs, and `inbox/` for [contributions](../architecture/team-tools.md#tools)). [Add a project](../features/add-project.md) creates it (`git init`, `project.toml`, a starter wiki). It is never a code repo: agents get no worktree of it, but one live checkout shared by the whole team, writable by the lead only ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

The project repo uses the project's `forge`, like the code repos, at the URL `forge.project_repo` names explicitly (any repo name, since `project` would clash between projects under one owner). Nothing is pushed on its own: the CEO pushes its `main` with **Sync** in [Project settings](../features/project-settings.md) ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). Foreman's own writes to it (contributions, the CEO's checkbox ticks and card moves) are committed directly, one commit per write ([Sandboxes and git policy](../architecture/sandboxes-and-git.md#project-knowledge-base)).

## Example

```toml
project = "ledger"
description = "Settlement and reconciliation service."
repos = ["ledger-api", "ledger-web"]   # optional; default is every git repo found in the folder
main = "main"                          # protected branch name
checks = "npm test"                    # run in a sandbox before handoff and on a merge request
merge_to_main = "ceo"                  # v1: always ceo
always_ask_ceo = [                     # gates: these always raise an approval request
  "new dependency",
  "touches shared staging",
  "scope change vs an accepted ADR",
]

[forge]
kind = "forgejo"                       # github | forgejo | gitea | none
url = "https://localhost:3000/ledger"  # org/owner base; repo name = folder name
project_repo = "https://localhost:3000/ledger/ledger-project"  # the project repo on the forge: any name
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `project` | String | yes | | slug | Project id. Used in instance ids (`agent@project`) and URLs |
| `description` | String | | | one line | Shown in the project header |
| `repos` | Array of String | | every git repo found in the folder, except `project/` | each a subfolder name of the project, never `project` | The code repos agents get worktrees of |
| `main` | String | | `main` | branch name | Protected branch. No agent writes it; it changes only by the CEO's decision: Merge to main, a PR merged on the forge, or the CEO's own git |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on a merge request ([Pipeline and merges](../features/pipeline-and-merges.md)) |
| `merge_to_main` | String | | `ceo` | enum: `ceo` | v1: always `ceo`, who chooses Merge to main (Foreman merges on that click) or Create PR and merges on the forge; Foreman never merges on its own |
| `forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | `none` = no forge: merge requests offer Merge to main and Reject, not Create PR ([Forge adapters](../architecture/forge-adapters.md)). A missing `forge` table means `none`; adoption writes it explicitly |
| `forge.url` | String (URL) | when `kind` ≠ `none` | | | Org/owner base URL; the repo name on the forge = repo folder name |
| `forge.project_repo` | String (URL) | when `kind` ≠ `none` | | | The [project repo](#the-project-repo) on the forge, given in full: its name is free, since `project` would clash between projects under one owner. Sync pushes `main` there |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |

The wiki has no field: it is always `project/wiki/`.

## Invariants

- The project folder name is the project id. `project` must equal it; a mismatch is a validation error.
- Foreman never writes this file outside the [Add a project](../features/add-project.md) adoption (initial write).
- No secret ever appears in this file. The forge token lives in [Secrets](./secrets.md).

## Invalid config

A `project.toml` that fails to parse or validate doesn't hide the project: it still appears, with an error banner on the project naming the file and the field. Foreman keeps the last good version in memory ([Persistence](../architecture/persistence.md#file-watching)); new deployments, and anything else that needs the invalid value, are refused; running deployments keep running.
