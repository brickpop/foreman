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
| `main` | String | | `main` | branch name | Protected branch. Nobody pushes it; it changes only by CEO merge |
| `checks` | String | | | shell command | Run in a sandbox before handoff and on PR ([Pipeline and pull requests](../features/pipeline-and-prs.md)) |
| `merge_to_main` | String | | `ceo` | enum: `ceo` | v1: always `ceo` |
| `forge.kind` | String | yes | | enum: `github`, `forgejo`, `gitea`, `none` | `none` = local-only PRs inside Foreman ([Forge adapters](../architecture/forge-adapters.md)) |
| `forge.url` | String (URL) | when `kind` ≠ `none` | | | Org/owner base URL; the repo name on the forge = repo folder name |
| `always_ask_ceo` | Array of String | | `[]` | | Gates. Hitting one makes the agent raise an approval [request](../features/requests.md). Listed in each [session prompt](../architecture/session-prompt.md) |

## Invariants

- The project folder name and `project` are the same slug. **Not yet decided**: see open questions.
- Foreman never writes this file outside the [Add a project](../features/add-project.md) adoption (initial write) and Chief of Staff edits on a branch.
- No secret ever appears in this file. The forge token lives in [Secrets](./secrets.md).

## Open questions

1. **Missing `forge`.** The prototype renders an adopted project without `forge` as Forgejo. Options: adoption writes `forge: {kind: none}` explicitly and a missing `forge` is a validation error shown in [Project settings](../features/project-settings.md); or a missing `forge` is treated as `none` ("Local only"). Recommendations differ: validation error (from this entry) vs treat as `none` (from Project settings); both agree that adoption writes `kind: none`.
2. **Folder name vs `project`.** If they differ, which is the id? Recommendation: the folder name is the id; `project` must equal it, else a validation error.
3. **Invalid config.** How does the UI surface a `project.yaml` that fails to parse or validate? Recommendation: the project still appears, with an error banner on the project naming the field; new deployments and other features that need the invalid value are refused; running deployments keep running.
