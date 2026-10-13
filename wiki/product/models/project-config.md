---
type: spec
status: planned
tags: [model, config, workspace, forge]
---

# Project config

`<project>/.foreman/project.yaml`: what a project is, which repos agents work on, the checks, the gates that always reach the CEO, and the forge that hosts PRs. It is committed with the project. The UI shows it **read-only**; changes are made in the CEO's editor or by the [Chief of Staff](../features/chief-of-staff.md) on a branch `foreman/config-N`, which comes back as a review request ([Project settings](../features/project-settings.md)). Deployments are **not** in this file: they are runtime state ([Deployment](./deployment.md)).

## Example

```yaml
project: ledger
description: Settlement and reconciliation service.
repos: [ledger-api, ledger-web]   # optional; default is every git repo found in the folder
wiki: ./wiki
main: main                        # protected branch name
checks: npm test                  # run in a sandbox before handoff and on PR
merge_to_main: ceo                # v1: always ceo
forge:
  kind: forgejo                   # github | forgejo | gitea | none
  url: https://localhost:3000/ledger   # org/owner base; repo name = folder name
always_ask_ceo:                   # gates: these always raise an approval request
  - new dependency
  - touches shared staging
  - scope change vs an accepted ADR
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `project` | String | yes | | slug | Project id. Used in instance ids (`agent@project`) and URLs |
| `description` | String | | | one line | Shown in the project header |
| `repos` | Array of String | | every git repo found in the folder | each a subfolder name of the project | The repos agents get clones of |
| `wiki` | String (path) | | `./wiki` | relative to the project folder | The [wiki bundle](../architecture/wiki-engine.md) |
| `main` | String | | `main` | branch name | Protected branch. Nobody pushes it; it changes only by the CEO's merge, on the forge or with git |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on PR ([Pipeline and pull requests](../features/pipeline-and-prs.md)) |
| `merge_to_main` | String | | `ceo` | enum: `ceo` | v1: always `ceo`, who merges on the forge (or with git without one); Foreman never merges |
| `forge.kind` | String | | `none` | enum: `github`, `forgejo`, `gitea`, `none` | `none` = no forge: the lead branch is published into the project repo for the CEO to review and merge with git ([Forge adapters](../architecture/forge-adapters.md)). A missing `forge` key means `none`; adoption writes it explicitly |
| `forge.url` | String (URL) | when `kind` ≠ `none` | | | Org/owner base URL; the repo name on the forge = repo folder name |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |

## Invariants

- The project folder name is the project id. `project` must equal it; a mismatch is a validation error.
- Foreman never writes this file outside the [Add a project](../features/add-project.md) adoption (initial write) and Chief of Staff edits on a branch.
- No secret ever appears in this file. The forge token lives in [Secrets](./secrets.md).

## Invalid config

A `project.yaml` that fails to parse or validate doesn't hide the project: it still appears, with an error banner on the project naming the file and the field. Foreman keeps the last good version in memory ([Persistence](../architecture/persistence.md#file-watching)); new deployments, and anything else that needs the invalid value, are refused; running deployments keep running.
