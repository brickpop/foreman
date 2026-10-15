---
type: task
status: done
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Instance manager: deploy, run, restart, re-attach

**Why:** the block that turns a deployment into running agents ([Building blocks → 5](../product/architecture/building-blocks.md), [Agent runtime](../product/architecture/agent-runtime.md)).

Blocked by [Podman engine](./podman-engine.md), [Worktrees per instance](./worktrees-per-instance.md), [Claude Code harness](./claude-harness-and-login.md), [State store](./state-store.md), and the decision on the journey's gaps (G1 kickoff; G4 is decided): [Decide the journey gaps](./decide-journey-gaps.md).

**Done when:** deploying a team (the [Deploy a team](../product/features/deploy-team.md) dialog) starts one instance per member (image, its worktrees and the project repo mounted, launch, token placeholder, start, terminal), statuses follow the instance lifecycle, crashes restart per policy, the concurrency queue holds, stopping keeps the worktrees on disk (deleting merged `wt/*` branches is `Git.Prune`, in the [Git epic](./epics/git.md)), and after a Foreman restart running containers are re-attached. `--demo-session` is removed ([technical debt](../technical-debt.md)).

- [x] the `engine: podman | direct` setting picks the engine at startup (Podman started first, an error before the URL is printed; `direct` prints a no-isolation warning; `just dev` sets `direct`)
- [x] the manager: `Deploy`, `Stop`, `List`, `Terminal`, `Restore`; statuses in the store and on the event stream (`instance.status`, `deployment.created`, `deployment.stopped`). Epic 1 statuses: queued, starting, running, stopped (idle, awaiting, done need the hooks: epic 2)
- [x] deploy and stop through the API (`POST /api/deployments`, `DELETE /api/deployments/:id`, `GET /api/instances` with statuses) and the client; the dialogs are the UI cards' ([Team and Agent pages](./ui-teams-agents.md), wired in [Teams and agents](./teams-and-agents.md))
- [x] restart policy (a crash restarts once, continuing its conversation; a second stops it) and the concurrency queue (`max_sessions`)
- [x] re-attach on boot (running containers by label; live instances without one restart; orphans stopped)
- Not here: why an instance stopped reaches only the server log until blocked requests exist ([Workflow](./epics/workflow.md))
