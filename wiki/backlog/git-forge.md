---
type: task
status: todo
priority: medium
tags: [feature]
epic: git
---

# The forge: tokens, Create PR, following the PR, Sync

**Why:** a project with a forge (GitHub, Forgejo, Gitea) can send a merge request there as a PR, and the reserved repos (the project repo, the global repo) sync with theirs ([Forge adapters](../product/architecture/forge-adapters.md), [Pipeline and merges](../product/features/pipeline-and-merges.md)).

**Landings and stacks (the CEO, 2026-11-11):** a merge request may land on a branch of its own ([Where a merge goes](../product/features/pipeline-and-merges.md#where-a-merge-goes)), and leads stack them (`main < f/a < f/b`). Create PR on such a review: decide whether the PR's head is the landing (`f/b`, pushed after it is made) and its base the target (`f/a`), so a stack becomes a forge's stacked PRs one to one.

**Done when:** Settings → Forge accounts checks and stores a token; a merge request offers **Create PR**, pushes the branch and opens the PR; Foreman follows it (merged, closed, review comments to the lead); **Sync** fetches, merges and pushes the project and workspace repos' `main`, stopping on a conflict; tests against a fake forge.

- [ ] tokens: `GET /api/secrets/forge`, `PUT /api/secrets/forge/:kind`; the real client in Settings
- [ ] Create PR; PR state polling; the PR's base is the merge request's target ([Merge targets](./archive/merge-targets.md)), not always the default branch; merged, closed, comments. A PR closed on the forge rejects its merge requests as Close does with the team working and the branches kept: the lead is told to stop work on it, and its work items stay with them ([Pipeline and merges](../product/features/pipeline-and-merges.md#review-and-merge))
- [ ] **PR-only projects** (the CEO, 2026-10-17): a project setting whose merge requests offer only Create PR; refused without a forge. Its key is to be named with the CEO: `merge_to_main` was retired on 2026-11-02 ([Merge targets](./archive/merge-targets.md)), and old files flag it as a problem. Specs: Project config
- [ ] Sync for the reserved repos
- [ ] **To decide with the CEO** (2026-11-06): Create PR pushes the lead branch, whose commits are the agents' and unsigned ([Merges into any branch](./archive/merge-into-any-branch.md)): a forge that requires signed commits on every branch refuses it (one squashed commit signed by the CEO instead?). And whether producing and integrating work, and distributing it (pushes, PRs), should be apart: "Merge means merge, not push"
- [ ] tests; specs
