---
type: spec
status: planned
tags: [architecture, workspace, runtime, config]
---

# Persistence

Where Foreman keeps what, and how changes on disk reach the UI. The principle (config vs. state vs. content) and the folder layout are in [Workspace](../models/workspace.md); this entry covers the mechanics.

## Stores

| Store | Location | Owner | Mechanics |
|---|---|---|---|
| Project config | `<project>/.foreman/project.yaml` | CEO / Chief of Staff | Read-only for Foreman except "Add a project" (creates it). Parsed and validated on load and on change ([Project config](../models/project-config.md)) |
| Wiki content | `<project>/wiki/` | agents, CEO | Read via the [wiki engine](./wiki-engine.md); only checkbox and card writes |
| Team definitions | `<workspace>/.foreman/teams/<team>.yaml` | Foreman (UI), Chief of Staff, editor | Read and written by Foreman ([Team definition](../models/team-definition.md)) |
| Workspace settings | `<workspace>/.foreman/settings.yaml` | Foreman (UI) | [Workspace settings](../models/workspace-settings.md) |
| Secrets | OS keychain or `<workspace>/.foreman/secrets` (0600) | Foreman (UI) | Write-only from the UI ([Secrets](../models/secrets.md)) |
| Runtime state | `<workspace>/.foreman/state.db` (SQLite) | Foreman | Deployments, instances, requests, ACP messages, pipeline items, read markers, UI prefs |
| Session logs | `<workspace>/.foreman/logs/<agent>@<project>/<session-id>.log` | Foreman | Raw PTY byte stream, for replay and scrollback |

### state.db

Minimum tables, one model entry each: [deployments](../models/deployment.md), [instances](../models/instance.md), [requests](../models/request.md), [acp_messages](../models/acp-message.md), [pipeline_items](../models/pipeline-item.md), and [read_markers / ui_prefs](../models/ui-state.md). Schema changes ship as ordered migrations applied at startup. Driver and migration tooling: [Tech stack](./tech-stack.md).

- Resolved requests stay in the table as history.
- On restart, deployments and instances are restored from here ([Agent runtime](./agent-runtime.md#recovery)).

### Session logs

- The supervisor appends every byte read from a session to its current log file before fanning it out ([Session runner](./session-runner.md)). A new session id (and file) per harness start.
- Scrollback for a newly attached terminal is replayed from the tail of the log, so the UI shows history after a reload or a server restart.
- The "last output line" shown in [Project settings](../features/project-settings.md) is derived from the stream (ANSI stripped), not stored separately.

## File watching

- The workspace indexer watches the workspace root (new or removed project folders and repos), every `project.yaml`, every wiki bundle, and `.foreman/teams/` and `settings.yaml`.
- Changes are debounced, reloaded and pushed to clients as events ([API](./api.md#events)) within ~1 s.
- A config file that fails to parse keeps its last good version in memory and surfaces the error (UI and server log) rather than breaking the project.
- Foreman's own writes to team files and settings also produce watch events; they are recognised (by content version) so they don't bounce back as "external change".

## Team file writes

Team definitions are edited from the UI ([Agent page](../features/agent-page.md)) while the Chief of Staff or the CEO's editor may edit the same file. Writes are atomic (temp file + rename) and guarded by the version the UI last read, like wiki writes; a conflict is reported, never silently overwritten. Writes should preserve comments and key order the CEO wrote.

## Open questions

1. **Log retention.** Session logs grow without bound (20+ sessions of raw PTY output). Options: keep forever; cap per instance (size or last N sessions) and rotate; delete with the deployment. Recommendation: rotate per instance at a size cap (e.g. 50 MB, newest kept; the cap a workspace setting), delete older files, and document that logs hold whatever the harness printed (see secrets in logs, [Secrets](../models/secrets.md#open-questions)).
