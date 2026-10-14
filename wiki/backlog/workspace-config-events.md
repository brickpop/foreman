---
type: task
status: todo
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Workspace and config loading, file watcher, event stream

**Why:** projects, team files and settings are files the CEO owns ([Workspace](../product/models/workspace.md)); the UI must follow them live within ~1 s, and every later screen needs the event stream.

**Done when:** Foreman lists projects (folders with `project/project.toml`), reads `project.toml`, team files and `settings.toml` with their validation errors, notices changes through the file watcher, and pushes them to the UI over `/api/events` (SSE); the UI's client interface exposes them.

- [ ] parse and validate [Project config](../product/models/project-config.md), [Team definition](../product/models/team-definition.md) (souls: [Agent soul](../product/models/agent-soul.md)), [Workspace settings](../product/models/workspace-settings.md)
- [ ] TOML everywhere (drop YAML); version-guarded writes of team files and settings
- [ ] the workspace repo: `git init` of `.foreman/` with its `.gitignore` (state.db, logs/, secrets, credentials/, access-token) on workspace creation and for existing workspaces; one commit per UI save (nothing pushed: the Sync action belongs to the [Git epic](./epics/git.md))
- [ ] fsnotify watcher, debounced; event bus; `GET /api/events` ([API → Events](../product/architecture/api.md#events))
- [ ] `/api/workspace`, `/api/projects`, `/api/teams` read endpoints; UI client methods
