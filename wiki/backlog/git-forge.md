---
type: task
status: todo
priority: medium
tags: [feature]
epic: git
---

# The forge: tokens, Create PR, following the PR, Sync

**Why:** a project with a forge (GitHub, Forgejo, Gitea) can send a merge request there as a PR, and the reserved repos (the project repo, the global repo) sync with theirs ([Forge adapters](../product/architecture/forge-adapters.md), [Pipeline and merges](../product/features/pipeline-and-merges.md)).

**Done when:** Settings → Forge accounts checks and stores a token; a merge request offers **Create PR**, pushes the branch and opens the PR; Foreman follows it (merged, closed, review comments to the lead); **Sync** fetches, merges and pushes the project and workspace repos' `main`, stopping on a conflict; tests against a fake forge.

- [ ] tokens: `GET /api/secrets/forge`, `PUT /api/secrets/forge/:kind`; the real client in Settings
- [ ] Create PR; PR state polling; merged, closed, comments
- [ ] **PR-only projects** (the CEO, 2026-10-17): a new `merge_to_main = "pr"` value, whose merge requests offer only Create PR; refused without a forge. Specs: Project config
- [ ] Sync for the reserved repos
- [ ] tests; specs
