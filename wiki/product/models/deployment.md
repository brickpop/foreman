---
type: spec
status: planned
tags: [model, teams, runtime]
---

# Deployment

A [team](./team-definition.md) assigned to a project with a goal. Runtime state kept by Foreman in `state.db`, never in `project.yaml` ([Persistence](../architecture/persistence.md)). A deployment gets its own [instances](./instance.md), sandboxes and ACP channel: agents in two deployments never mix, even when they share a soul. Created from [Deploy a team](../features/deploy-team.md), stopped from the [Team page](../features/team-page.md).

## Schema

Table `deployments`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique | The prototype keys it `team@project` |
| `team_id` | String | yes | | a team file id | |
| `project_id` | String | yes | | a project id | |
| `goal` | String | yes | | non-blank | "What does done look like?" Part of every member's [session prompt](../architecture/session-prompt.md) |
| `status` | String | yes | `active` | enum: `active`, `stopped` | |
| `created_at` | Timestamp | yes | now | | |
| `stopped_at` | Timestamp | | | set when stopped | |

## Lifecycle

1. **Deploy**: create the deployment, then one instance per team member in `starting`. Sandboxes are created and sessions start ([Agent runtime](../architecture/agent-runtime.md)).
2. **Run**: members' statuses follow their harness and ACP activity. Hiring onto the team starts a new instance on every active deployment.
3. **Stop**: terminate the deployment's processes and keep sandboxes and worktrees on disk. Other deployments of the same team are not affected.
4. **Server restart**: active deployments are restored from `state.db` and their sessions restarted with the last task note.

## Invariants

- At most one **active** deployment per (team, project) pair. The Deploy dialog disables already-deployed pairs.
- ACP is scoped to a deployment: `nyx@ledger` cannot message `nyx@relay`.
- Stopping one deployment never touches another.

## Open questions

1. **What happens to open requests and pipeline items on stop?** The prototype drops them from view. Options: expire open requests (status `expired`, kept in history) and keep pipeline items visible as stalled; hide both; block stop while a review is open. Recommendation: expire requests, keep pipeline items (with an open PR still on the forge) visible in Changes marked as "stopped".
2. **Re-deploying a stopped pair.** A new row, or reactivate the old one (reusing its `req-N` counter, ACP history and sandboxes)? Recommendation: reactivate the same row, keeping history and sandboxes.
3. **Stop confirmation.** The prototype stops immediately, from the [Team page](../features/team-page.md). Recommendation: confirm with a dialog stating how many sessions end and that worktrees are kept; the Stop button uses the destructive style ([Design system](../design/design-system.md#open-questions)).
