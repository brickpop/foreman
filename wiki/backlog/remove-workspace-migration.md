---
type: task
status: blocked
priority: low
tags: [chore]
epic: install-first-run
---

# Remove the workspace migration once the CEO has migrated

**Why:** the migration of [Project workspaces](./project-workspaces.md) (moving a project out of `~/.foreman/projects/` and `~/.foreman/sandboxes/`, carrying the agents' conversations over) exists only for the CEO's projects made before workspaces. Foreman has no other users yet, so once those are moved it is dead code, along with the old layout's paths it has to know (the CEO, 2026-10-28). Listed in [Technical debt](../technical-debt.md#the-workspace-migration).

Blocked by [Project workspaces](./project-workspaces.md), then by the CEO saying every project is migrated.

**Done when:** the migration's code, route, UI offer and tests are gone, nothing in the code names `projects/` or `sandboxes/` under the home any more, the specs no longer mention it, and its technical-debt item is removed.

- [ ] the CEO confirms every project is migrated
- [ ] remove the server's migration (the moves, the adapters' conversation renames), its route and types, the Project settings offer, and their tests
- [ ] remove the old layout's leftovers: a home's `projects/` or `sandboxes/` is just an unknown folder
- [ ] the specs (Home, Project settings, Harness adapters, API) and the technical-debt item
