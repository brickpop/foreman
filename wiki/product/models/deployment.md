---
type: spec
status: planned
tags: [model, teams, runtime]
---

# Deployment

A [team](./team-definition.md) assigned to a project, with an optional goal. Runtime state kept by Foreman in `state.db`, never in `project.toml` ([Persistence](../architecture/persistence.md)). A deployment gets its own [instances](./instance.md), worktrees and team-message channel: agents in two deployments never mix, even when they share a soul. Created from [Deploy a team](../features/deploy-team.md), stopped from the [Team page](../features/team-page.md).

## Schema

Table `deployments`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | The prototype keys it `team@project` |
| `team_id` | String | yes | | a team file id | |
| `project_id` | String | yes | | a project id | |
| `goal` | String | no | `""` | trimmed | "What does done look like?" Part of every member's [session prompt](../architecture/session-prompt.md). Empty when the CEO gives it in the lead's session instead (the CEO, 2026-10-16); the lists show `No goal set` |
| `status` | String | yes | `active` | enum: `active`, `stopped` | |
| `created_at` | Timestamp | yes | now | | |
| `stopped_at` | Timestamp | | | set when stopped | |

## Lifecycle

1. **Deploy**: create the deployment, then one instance per team member in `starting`. Worktrees are created and sessions start ([Agent runtime](../architecture/agent-runtime.md)).
2. **Run**: members' statuses follow their harness and team-message activity. Hiring onto the team starts a new instance on every active deployment.
3. **Stop** (after the CEO confirms, see [Team page](../features/team-page.md)): terminate the deployment's processes and keep its worktrees on disk; its agents' `wt/*` branches already merged into `main` are deleted, the rest kept ([Sandboxes and git policy → Branches](../architecture/sandboxes-and-git.md#branches)). Other deployments of the same team are not affected. Its open [requests](./request.md) become `expired` (kept in history); its [work items](./work-item.md) and [merge requests](./merge-request.md) stay visible in Changes, marked "stopped" (an open PR stays open on the forge, and Foreman still notices when it is merged).
4. **Server restart**: active deployments are restored from `state.db` and their sessions restarted with the last task note.

## Invariants

- At most one **active** deployment per (team, project) pair. The Deploy dialog disables already-deployed pairs.
- At most one row per (team, project) pair: re-deploying a stopped pair **reactivates** that row (new goal, `status: active`, `stopped_at` cleared), keeping its `msg-N` counter, team-message history and worktrees.
- Team messaging is scoped to a deployment: `nyx@ledger` cannot message `nyx@relay`.
- Stopping one deployment never touches another.
