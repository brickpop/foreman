---
type: task
status: done
priority: high
tags: [bug]
epic: workflow
---

# Continue conversations on redeploy

**Why:** stopping a team and deploying it again on the same project loses every agent's conversation with the CEO: "losing them is bad, bad" (the CEO, 2026-11-02). Nothing is deleted. Each agent's harness folder (`~/.foreman/harness/<project>/<agent>/`) outlives deployments, but a new deployment's first start is treated as fresh and skips them on purpose.

**Cause (checked 2026-11-02):**
- `Manager.Deploy` puts a new instance row under the same id (`agent@project`), so `StartedAt` is zero again.
- `launch.go` sets `Continue: !first && !switched`, where `first` is `inst.StartedAt.IsZero()`.
- On a non-continuing start, the adapter writes `foreman-fresh-start` with the conversations already there, and `Conversations.Continues` never continues one of those (`internal/harness/conversations.go`, Claude Code and Pi).

**Decided (the CEO, 2026-11-02):** a redeploy continues each agent's last conversation on the project by default. [Deploy a team](../../product/features/deploy-team.md) shows **Start fresh** (off) only when earlier conversations exist; old ones always stay on disk. Applying a team change to a running team is [Reload](../../product/models/team-definition.md#reload), never Stop and deploy again ([The lead proposes changes to its own team](../lead-proposes-team.md)).

**Already right, keep it so:** Restart, a crash's automatic restart, a Foreman restart and a wake all continue. Another harness still starts fresh ([Agent runtime → Restarts](../../product/architecture/agent-runtime.md#restarts)).

- [x] `POST /api/deployments` takes `fresh` (default false); Deploy sets each instance's `fresh` (migration 016), cleared at its first start
- [x] the first start continues unless `fresh` (or another harness, or **another team**); the fresh-start list is written only when starting fresh
- [x] another team: agent ids are only unique within a team ([Agents share ids across teams](./agents-share-ids-across-teams.md)), so a later team's `mara` on the project gets the same harness folder. Read from the store, no file in the folder: the agent's latest instance on the project (`agent@project`, its last user) must be of this team, on this harness, and started
- [x] `GET /api/deployments/continuing?team=&project=` (`Manager.Continuing`, the same rule Deploy uses)
- [x] the dialog's checkbox ([Deploy a team](../../product/features/deploy-team.md))
- [x] what a continued first start gets: no opener and no "your worktrees hold earlier work" notice (it remembers), like a restart; check that this holds for the lead's goal
- [x] tests: `TestRedeployContinues` (stop then deploy continues the lead; a member that slept starts fresh, then counts; Start fresh, then a restart continues the fresh one; another team's same-id lead starts fresh, and so does the first team after it); another harness is `TestRestartTakesTheProfileAsItIsNow` and `Continuing`'s harness check; Pi before a Start fresh is `foreman-fresh-start`, unchanged (`internal/harness`); server: `fresh` passed, `continuing` an empty list never null, its errors; client: the body and the query
- [x] specs as built: [Agent runtime → Restarts](../../product/architecture/agent-runtime.md#restarts), [Deploy a team](../../product/features/deploy-team.md), [API](../../product/architecture/api.md)
