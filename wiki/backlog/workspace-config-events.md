---
type: task
status: todo
priority: high
tags: [feature]
---

# Workspace and config loading, file watcher, event stream

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** projects, team files and settings are files the CEO owns ([Workspace](../product/models/workspace.md)); the UI must follow them live within ~1 s, and every later screen needs the event stream.

**Done when:** Foreman lists projects, reads `project.yaml`, team files and `settings.yaml` with their validation errors, notices changes through the file watcher, and pushes them to the UI over `/api/events` (SSE); the UI's client interface exposes them.

- [ ] parse and validate [Project config](../product/models/project-config.md), [Team definition](../product/models/team-definition.md) (souls: [Agent soul](../product/models/agent-soul.md)), [Workspace settings](../product/models/workspace-settings.md)
- [ ] version-guarded writes of team files (comments preserved)
- [ ] fsnotify watcher, debounced; event bus; `GET /api/events` ([API → Events](../product/architecture/api.md#events))
- [ ] `/api/workspace`, `/api/projects`, `/api/teams` read endpoints; UI client methods
