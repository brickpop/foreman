---
type: task
status: blocked
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Instance manager: deploy, run, restart, re-attach

**Why:** the block that turns a deployment into running agents ([Building blocks → 5](../product/architecture/building-blocks.md), [Agent runtime](../product/architecture/agent-runtime.md)).

Blocked by [Podman engine](./podman-engine.md), [Worktrees per instance](./worktrees-per-instance.md), [Claude Code harness](./claude-harness-and-login.md), [State store](./state-store.md), and the decision on the journey's gaps (G1 kickoff, G4 screen after a restart): [Decide the journey gaps](./decide-journey-gaps.md).

**Done when:** deploying a team (the [Deploy a team](../product/features/deploy-team.md) dialog) starts one instance per member (image, its worktrees and the project repo mounted, launch, token placeholder, start, terminal), statuses follow the instance lifecycle, crashes restart per policy, the concurrency queue holds, stopping keeps the worktrees on disk (deleting merged `wt/*` branches is `Git.Prune`, in the [Git epic](./epics/git.md)), and after a Foreman restart running containers are re-attached. `--demo-session` is removed ([technical debt](../technical-debt.md)).

- [ ] `Instances` interface: `Ensure`, `Status`, `Terminal`, `Watch`
- [ ] deploy and stop (API + dialogs), statuses on the event stream
- [ ] restart policy, concurrency queue
- [ ] re-attach on boot
