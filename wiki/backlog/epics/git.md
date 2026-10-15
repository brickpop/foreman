---
type: epic
status: todo
order: 3
---

# Git

**Goal:** the plumbing merges need. Worktrees and branches per agent already exist since [Terminal multiplexer](./terminal-multiplexer.md), and the git tools (`switch`, `commit`, `update`) and hand-offs that name a branch since [Messaging and hand-offs](./messaging-handoffs.md); this epic turns hand-offs and integrations into tracked work on their way to `main`.

What it builds ([Sandboxes and git policy](../../product/architecture/sandboxes-and-git.md); [Building blocks → Git](../../product/architecture/building-blocks.md#8-git-git); [Pipeline and merges](../../product/features/pipeline-and-merges.md)):
- **work items** from hand-offs, and **integration detection**: an engineer's branch contained in a lead branch (`Git.Contains`);
- the project **checks**, run in a throwaway container before a hand-off is accepted and on `request_merge`;
- **local merge**: the CEO's Merge to main (`Git.Merge`), then deleting merged branches under the existing rule (`Git.Prune`, when a deployment stops);
- the **optional forge PR**: the CEO's Create PR pushes the branch and opens it (`Git.Push`, the Forge adapter), and fetching `main` back after a merge there (`Git.Fetch`);
- **merged detection**: a merge request is merged when `main` contains its branch head, whoever merged it (Foreman, the forge, or the CEO's own git);
- **Sync** for the reserved repos: on the CEO's click, fetch the forge's `main`, merge it in, then push the workspace repo's or the project repo's `main`; on a conflict, stop and tell the CEO;
- the branch policy checks on `handoff` and `request_merge` (members on `wt/<agent>/*`, leads on `lead/<team>/*`, nobody on `main`).
- **merge request reads**: `GET /api/merge-requests/:id` and `GET /api/merge-requests/:id/diff` (the change the CEO reads before deciding; the screens say "Not built yet" until then);
- **forge tokens**: `GET /api/secrets/forge` and `PUT /api/secrets/forge/:kind` (check the token against the forge, store it as a [secret](../../product/models/secrets.md), write `forges.<kind>.url`), then Settings → Forge accounts switches from the demo client to the real one, and Sync is enabled;

**To decide in this epic:**

- [ ] how Merge to main relates to the CEO's own checkout when it has `main` checked out ([Pipeline and merges → Open questions](../../product/features/pipeline-and-merges.md#open-questions))

**Usable when done:** work items and branches show in Changes → In flight, with their checks; an integrated lead branch can be merged to `main` from Foreman, or sent to the forge as a PR.

**Journey steps:** 16 and 17 (work items, checks, integration), and the merge and PR in 19, of [the journey](../../product/journey.md).

**Blocked by:** [Messaging and hand-offs](./messaging-handoffs.md).

**Exit criteria:** a member's hand-off becomes a work item with its checks, the lead's merge marks it integrated, and the lead branch can be merged to `main` locally or through a PR on a forge, with tests on the fake harness.
