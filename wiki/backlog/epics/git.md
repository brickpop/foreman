---
type: epic
status: todo
order: 3
---

# Git

**Goal:** each agent works on its own branch, in its own git worktree, and branches flow from engineers to the lead and on to PRs. Until this epic, every agent of a deployment edits the one shared project folder ([Terminal multiplexer](./terminal-multiplexer.md)).

What it builds ([Sandboxes and git policy](../../product/architecture/sandboxes-and-git.md), the target design; [Building blocks → Git](../../product/architecture/building-blocks.md)):
- a **git worktree and branch per agent**, created from the mounted project repo; worktrees work inside containers because the repo's `.git` is mounted with it;
- **hand-offs as branches:** no fetching between clones is needed, since every worktree shares one repo;
- **integration detection:** an engineer's branch contained in a lead branch;
- **pushing lead branches** to the forge, and the rest of the plumbing PRs need;
- the branch policy (members on `wt/<agent>/*`, leads on `lead/<team>/*`, nobody on `main`).
- **removing the turns per project** (Messaging's gate and the Sessions warning): with a worktree each, agents of one project work in parallel.

**Usable when done:** agents no longer collide in one working tree; the CEO's checkout stays on its branch; work items and branches show in Changes → In flight.

**Journey steps:** 9.1, 16, 17 and the push in 18 of [the journey](../../product/journey.md).

**Blocked by:** [Messaging and hand-offs](./messaging-handoffs.md), and the decision on the Git block (gap G5 of the journey).

**Exit criteria:** two engineers and a lead work in parallel on one project without touching each other's files or the CEO's checkout, and an integrated lead branch is ready to push.
