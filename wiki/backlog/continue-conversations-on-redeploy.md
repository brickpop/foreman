---
type: task
status: todo
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

**Decided (the CEO, 2026-11-02):** a redeploy continues each agent's last conversation on the project by default. [Deploy a team](../product/features/deploy-team.md) shows **Start fresh** (off) only when earlier conversations exist; old ones always stay on disk. Applying a team change to a running team is [Reload](../product/models/team-definition.md#reload), never Stop and deploy again ([The lead proposes changes to its own team](./lead-proposes-team.md)).

**Already right, keep it so:** Restart, a crash's automatic restart, a Foreman restart and a wake all continue. Another harness still starts fresh ([Agent runtime → Restarts](../product/architecture/agent-runtime.md#restarts)).

- [ ] `POST /api/deployments` takes `fresh` (default false); the deployment carries it to its instances' first start
- [ ] the first start continues unless `fresh` (or another harness, or **another team**); the fresh-start list is written only when starting fresh
- [ ] another team: agent ids are only unique within a team ([Agents share ids across teams](./archive/agents-share-ids-across-teams.md)), so a later team's `mara` on the project gets the same harness folder. It must start fresh: the folder needs to record which team its last conversation was with
- [ ] a way for the dialog to know there are earlier conversations for the team's agents on the project (`Conversations` already lists them)
- [ ] the dialog's checkbox ([Deploy a team](../product/features/deploy-team.md))
- [ ] what a continued first start gets: no opener and no "your worktrees hold earlier work" notice (it remembers), like a restart; check that this holds for the lead's goal
- [ ] tests: stop then deploy continues each agent, including the asleep members once woken; Start fresh doesn't and keeps the files; another harness starts fresh; another team on the project is untouched; Pi doesn't pick up a conversation from before a Start fresh
- [ ] specs as built: [Agent runtime → Restarts](../product/architecture/agent-runtime.md#restarts), [Deploy a team](../product/features/deploy-team.md), [API](../product/architecture/api.md)
