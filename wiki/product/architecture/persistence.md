---
type: spec
status: partial
tags: [architecture, workspace, runtime, config]
---

# Persistence

Where Foreman keeps what, and how changes on disk reach the UI. The principle (config vs. state vs. content) and the folder layout are in [Workspace](../models/workspace.md); this entry covers the mechanics.

## Stores

| Store | Location | Owner | Mechanics |
|---|---|---|---|
| Project config | `<project>/project/project.toml` (project repo) | CEO / Chief of Staff | Read-only for Foreman except "Add a project" (creates it). Parsed and validated on load and on change ([Project config](../models/project-config.md)) |
| Wiki content | `<project>/project/wiki/` (project repo) | lead (commit through `commit()` (no review)), CEO | Read via the [wiki engine](./wiki-engine.md); Foreman writes only checkboxes, card moves and `contribute` notes, one commit per write |
| Team definitions | `<workspace>/.foreman/teams/<team>.toml` (workspace repo) | Foreman (UI), Chief of Staff (on a branch), editor | Read and written by Foreman, one commit per save ([Team definition](../models/team-definition.md)) |
| Workspace settings | `<workspace>/.foreman/settings.toml` (workspace repo) | Foreman (UI) | [Workspace settings](../models/workspace-settings.md) |
| Secrets | OS keychain or `<workspace>/.foreman/secrets` (0600) | Foreman (UI) | Write-only from the UI ([Secrets](../models/secrets.md)) |
| Runtime state | `<workspace>/.foreman/state.db` (SQLite) | Foreman | Deployments, instances, requests, team messages, work items, merge requests, read markers, UI prefs |
| Session logs | `<workspace>/.foreman/logs/<agent>@<project>/<session-id>.log` | Foreman | Raw terminal byte stream: each session's history and audit trail (a reload shows a snapshot instead, [Terminal state](./terminal-state.md)) |

### state.db

Minimum tables, one model entry each: [deployments](../models/deployment.md), [instances](../models/instance.md), [requests](../models/request.md), [team_messages](../models/team-message.md), [work_items](../models/work-item.md), [merge_requests](../models/merge-request.md), and [read_markers / ui_prefs](../models/ui-state.md). Schema changes ship as ordered migrations applied at startup. Driver and migration tooling: [Tech stack](./tech-stack.md).

- Resolved requests stay in the table as history.
- On restart, deployments and instances are restored from here ([Agent runtime](./agent-runtime.md#recovery)).

### Session logs

- The Terminal block appends every byte read from a session to its current log file before fanning it out ([Building blocks](./building-blocks.md#2-terminal-terminal)). A new session id (and file) per harness start.
- Scrollback for a newly attached terminal is replayed from the tail of the log, so the UI shows history after a reload or a server restart.
- **Retention:** logs rotate per instance at a size cap (50 MB by default, a [workspace setting](../models/workspace-settings.md)); the oldest files beyond it are deleted. Logs hold whatever the harness printed, minus the secrets Foreman redacts ([Secrets](../models/secrets.md#invariants)).
- The "last output line" shown in [Project settings](../features/project-settings.md) is derived from the stream (ANSI stripped), not stored separately.

## File watching

- The workspace indexer watches the workspace root (new or removed project folders and repos), every `project/project.toml`, every wiki bundle, and `.foreman/teams/` and `settings.toml`.
- Changes are debounced, reloaded and pushed to clients as events ([API](./api.md#events)) within ~1 s.
- A config file that fails to parse keeps its last good version in memory and surfaces the error (UI and server log) rather than breaking the project.
- Foreman's own writes to team files and settings also produce watch events; they are recognised (by content version) so they don't bounce back as "external change".

## Team file writes

Team definitions are edited from the UI ([Agent page](../features/agent-page.md)) while the Chief of Staff or the CEO's editor may edit the same file. Writes are atomic (temp file + rename) and guarded by the content hash the UI last read, like wiki writes; a conflict is reported, never silently overwritten. A write re-encodes the whole TOML file, so comments the CEO wrote by hand don't survive it; each save is then **one commit** in the workspace repo, which keeps every earlier version. The same holds for settings ([Team definition](../models/team-definition.md#writes)).
