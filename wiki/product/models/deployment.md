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
| `stop_reason` | String | no | `""` | | Why Foreman stopped it on its own (`ledger is gone: all its code repos are missing`); empty when the CEO stopped it, and cleared by a redeploy. Shown on the project's [Activity](../features/project-activity.md) tab, under Older sessions |

## Lifecycle

1. **Deploy**: create the deployment, then one instance per team member in `starting`. Worktrees are created and sessions start ([Agent runtime](../architecture/agent-runtime.md)).
2. **Run**: members' statuses follow their harness and team-message activity. Hiring onto the team starts a new instance on every active deployment.
3. **Stop** (after the CEO confirms, see [Team page](../features/team-page.md)): terminate the deployment's processes and keep its worktrees on disk; its agents' `wt/*` branches already merged into `main` are deleted, the rest kept ([Sandboxes and git policy → Branches](../architecture/sandboxes-and-git.md#branches)). Other deployments of the same team are not affected. Its open [requests](./request.md) become `expired` (kept in history); its [work items](./work-item.md) and [merge requests](./merge-request.md) stay visible in the project's [Repositories](../features/repositories.md) tab, marked "stopped" (an open PR stays open on the forge, and Foreman still notices when it is merged).
4. **Stopped by Foreman, its project gone**: when a check finds every code repo of the project missing, or its project repo folder gone, Foreman checks again 60 s later (a move or a git operation may be in progress). Still gone, each active deployment on the project is stopped as in 3, with the reason in `stop_reason` and a toast ([Notifications](../features/notifications.md)). One repo of several missing only blocks starts. A project with no code repo is never gone. Checks run on load, on each reload of the projects and after each project change; code repos aren't watched, so a deleted repo is noticed at the next of those, or when an agent starts (the CEO, 2026-10-20).
5. **Server restart**: active deployments are restored from `state.db` and their sessions restarted with the last task note.

## Invariants

- At most one **active** deployment per (team, project) pair. The Deploy dialog disables already-deployed pairs.
- At most one row per (team, project) pair: re-deploying a stopped pair **reactivates** that row (new goal, `status: active`, `stopped_at` and `stop_reason` cleared), keeping its `msg-N` counter, team-message history and worktrees.
- Team messaging is scoped to a deployment: `nyx@ledger` cannot message `nyx@relay`.
- Stopping one deployment never touches another.
