---
type: task
status: done
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Wire Team and Agent pages to the real backend: souls through team files

**Why:** the pages exist on the demo client ([Team and Agent pages](./ui-teams-agents.md)); this card reads and writes the real team files ([Team page](../product/features/team-page.md), [Agent page](../product/features/agent-page.md), [Team definition](../product/models/team-definition.md), [Deploy a team](../product/features/deploy-team.md)).

Blocked by [Team and Agent pages](./ui-teams-agents.md) and [Workspace and config](./workspace-config-events.md).

**Done when:** teams come from `.foreman/teams/*.toml`; soul edits are written back (version-guarded, one commit per save in the workspace repo); the session prompt is composed by the server; Deploy creates a real deployment (with the [Instance manager](./instance-manager.md)); these areas leave the demo client.

- [x] keep the roster order of the team file: `TeamConfig.Order` (decoded from the file, sent as `order`, written back in that order by `encodeTeam`); `TeamConfig.Roster()` (lead first, then the file's order) replaces the two sorted copies in `prompt` and `instances`, and the UI's `roster()` mirrors it for Sessions, Members and the demo
- [x] teams and souls API: `GET /api/teams`, `PUT /api/teams/:id/agents/:agent` (version-guarded; 409 / 400 like settings, through one `writeSaveError`; the loaded team is copied before the change); the Agent page's Keep mine / Take theirs already handles a conflict
- [x] composed session prompt endpoint (`GET /api/instances/:id/prompt`, and the template)
- [x] waits for the CEO's review of the whole UI on the demo client ([Full UI demo](./epics/full-ui-demo.md)): approved 2026-10-15
- [x] lazy starts (journey G1): only the lead starts on deploy, with the goal as its harness's opening message (`harness.Session.Opener`; Claude Code's prompt argument); the others are `asleep` (migration 002) until `Manager.Wake` (`POST /api/instances/:id/wake`, the pane's Start)
- [x] Deploy and Stop on the Instance manager (`POST /api/deployments`, `DELETE /api/deployments/:id`), checked end to end with the fake harness; `--demo-session` removed, `just dev` deploys its sample team on `--fake-harness`
- [x] every area on the real client, no badges ([Going live](./epics/full-ui-demo.md#going-live))
