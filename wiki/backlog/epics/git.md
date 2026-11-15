---
type: epic
status: in-progress
order: 3
---

# Git

**Goal:** the plumbing merges need. Worktrees and branches per agent already exist since [Terminal multiplexer](./terminal-multiplexer.md), and the git tools (`switch`, `commit`, `merge`) and hand-offs that name a branch since [Messaging and hand-offs](./messaging-handoffs.md); this epic turns hand-offs and integrations into tracked work on their way to `main`.

What it builds ([Sandboxes and git policy](../../product/architecture/sandboxes-and-git.md); [Building blocks → Git](../../product/architecture/building-blocks.md#8-git-git); [Pipeline and merges](../../product/features/pipeline-and-merges.md)):
- **work items** from hand-offs, and **integration detection**: an engineer's branch contained in a lead branch (`Git.Contains`);
- the project **checks**, run in a throwaway container before a hand-off is accepted and on `request_merge`;
- **local merge**: the CEO's Merge into a branch of the CEO's (`Git.Squash`, signed as the CEO's git config says; into any branch but an agent's since 2026-11-06, [Merges into any branch](../archive/merge-into-any-branch.md), after [Merge targets](../archive/merge-targets.md)), then deleting merged branches under the existing rule (`Git.Prune`, when a deployment stops);
- **merged detection**: a merge request is merged when `main` contains its branch head, whoever merged it (Foreman, or the CEO's own git, a PR on a forge included);
- the branch policy checks on `handoff` and `request_merge` (members on `wt/<agent>/*`, leads on `lead/<team>/*`, nobody on `main`).
- **merge request reads**: `GET /api/merge-requests/:id` and `GET /api/merge-requests/:id/diff` (the change the CEO reads before deciding; the screens say "Not built yet" until then);

**To decide in this epic:**

- [x] how Merge to main relates to the CEO's own checkout (decided 2026-10-17: [Pipeline and merges → Merge to main](../../product/features/pipeline-and-merges.md#merge-to-a-target))

**Usable when done:** work items and branches show in Changes → In flight, with their checks; an integrated lead branch can be merged to `main` from Foreman. Foreman never talks to a forge ([Remove the forge](../archive/remove-forge.md), the CEO, 2026-11-15).

**Journey steps:** 15 and 16 (work items, checks, integration), and the merge in 18, of [the journey](../../product/journey.md).

**Blocked by:** [Messaging and hand-offs](./messaging-handoffs.md).

**Exit criteria:** a member's hand-off becomes a work item with its checks, the lead's merge marks it integrated, and the lead branch can be merged to `main` locally, with tests on the fake harness.
