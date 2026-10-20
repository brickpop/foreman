---
type: task
status: todo
priority: low
tags: [feature]
epic: projects-anywhere
---

# Stop a deployment whose project folder was deleted

**Folded into [Projects anywhere](./epics/projects-anywhere.md) (2026-10-20):** projects become records whose repos can go missing; a deployment whose repos all vanished stops there. Kept for its open points, decided with that epic.

**Why (the CEO, 2026-10-17, very low priority):** when the CEO deletes a project's folder from the workspace while a team is deployed on it, the deployment is left pointing at nothing: its agents' worktrees and the project repo are gone under them. Foreman already notices the project disappearing (`workspace.changed`, [API → Events](../product/architecture/api.md#events)); it should then undeploy, as **Stop** does ([Deployment → Lifecycle](../product/models/deployment.md#lifecycle)).

**To decide when the card starts:**
- Stop at once, or only after the folder has stayed gone for a while (a folder briefly missing during a move or a `git` operation shouldn't stop a team).
- What Stop's steps mean with no folder left: worktrees and `wt/*` branches are already gone with it, so only the processes, requests and records remain.
- How the CEO is told (a toast, the deployment marked as stopped with the reason).

**Done when:** deleting a deployed project's folder stops its deployments on its own, with the reason shown; a test covers it.

- [ ] decide the points above with the CEO
- [ ] spec (Deployment lifecycle), the stop on the project's disappearance, tests
