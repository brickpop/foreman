---
type: task
status: blocked
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Wire Team and Agent pages to the real backend: souls through team files

**Why:** the pages exist on the demo client ([Team and Agent pages](./ui-teams-agents.md)); this card reads and writes the real team files ([Team page](../product/features/team-page.md), [Agent page](../product/features/agent-page.md), [Team definition](../product/models/team-definition.md), [Deploy a team](../product/features/deploy-team.md)).

Blocked by [Team and Agent pages](./ui-teams-agents.md) and [Workspace and config](./workspace-config-events.md).

**Done when:** teams come from `.foreman/teams/*.toml`; soul edits are written back (version-guarded, one commit per save in the workspace repo); the session prompt is composed by the server; Deploy creates a real deployment (with the [Instance manager](./instance-manager.md)); these areas leave the demo client.

- [ ] teams and souls API (read, version-guarded write)
- [ ] composed session prompt endpoint
- [ ] Deploy wired to the Instance manager
- [ ] swap the areas in main.tsx; remove their badges
