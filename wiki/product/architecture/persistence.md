---
type: spec
status: partial
tags: [architecture, home, runtime, config]
---

# Persistence

Where Foreman keeps what, and how changes on disk reach the UI. The principle (config vs. state vs. content) and the folder layout are in [Home](../models/home.md): what the CEO owns is in [Foreman's folder](../models/home.md#foremans-folder) (`<folder>`, `~/foreman` by default), Foreman's machinery in its home, `~/.foreman/`. This entry covers the mechanics.

## Stores

| Store | Location | Owner | Mechanics |
|---|---|---|---|
| Project records | `<folder>/hq/projects.toml` (Foreman's HQ) | Foreman (UI), editor | Every project's id, display name, hidden flag, its HQ's origin and code repos (name, origin, root commit, default branch): what travels between machines. Written version-guarded, one commit per change; ids unique in any case; every folder of a project follows from Foreman's folder and its id ([Project config → projects.toml](../models/project-config.md#projectstoml)) |
| This machine's part of each project | `state.db`, tables `projects` and `project_repos` | Foreman (UI) | Its profile and each code repo's path ([Project config → The project record](../models/project-config.md#the-project-record)) |
| Project config | `<folder>/projects/<project>/hq/project.toml` (the project's HQ) | CEO / Chief of Staff | Read-only for Foreman except New project (creates it). Parsed and validated on load and on change ([Project config](../models/project-config.md)) |
| Wiki content | `<folder>/projects/<project>/hq/` (the project's HQ, a wiki at its root) | lead (commit through `commit()` (no review)), CEO | Read via the [wiki engine](./wiki-engine.md); Foreman writes only checkboxes, card moves and `contribute` notes, one commit per write |
| Team definitions | `<folder>/hq/teams/<team>.toml` (Foreman's HQ) | Foreman (UI, Chief of Staff proposals the CEO confirms), editor | Read and written by Foreman, one commit per save ([Team definition](../models/team-definition.md)) |
| The CEO's instructions | `<folder>/hq/instructions.md` (Foreman's HQ) | Foreman (UI), editor | Prose, read on load and on change; written by Foreman version-guarded, one commit per save ([Settings → Instructions for every agent](../features/settings.md#instructions-for-every-agent)) |
| Global settings | `~/.foreman/settings.toml` (no git: this machine's, the look included) | Foreman (UI), editor | Written version-guarded, never committed; `foreman_folder` is read at start only ([Global settings](../models/global-settings.md)) |
| Project files | `<folder>/projects/<project>/files/` (`inbox/`, `shared/`) | CEO, agents | Not git; never read by Foreman ([Sandboxes and git policy → Layout](./sandboxes-and-git.md#layout)) |
| Agents' worktrees | `<folder>/worktrees/<project>/<agent>/<repo>/` | agents (through Foreman's git) | Disposable: every commit is in the code repo ([Sandboxes and git policy](./sandboxes-and-git.md#layout)) |
| Runtime state | `~/.foreman/state.db` (SQLite) | Foreman | This machine's part of each project, deployments, instances, requests, team messages, work items, merge requests, read markers, UI prefs |
| Session logs | `~/.foreman/logs/<agent>@<project>/<session-id>.log` | Foreman | Raw terminal byte stream: each session's history and audit trail (a reload shows a snapshot instead, [Terminal state](./terminal-state.md)) |
| The CEO's, on this machine (every project) | The home, `~/.foreman/` (`internal/userdata`; `--home` for tests and `just dev`): `profiles/<name>/` (a harness and its login), `images/` (the agent images' build lock, state and log), `server.lock` | Foreman (UI) | Read as it is when asked ([Settings → Agents → Profiles](../features/settings.md#profiles)). **One Foreman per machine** (the CEO, 2026-10-20): `server.lock` (0600) holds an OS lock as long as Foreman runs, released by the system however it ends, and the URL it serves; a second `foreman` prints that URL, opens it, and exits ([Projects anywhere](../../backlog/epics/projects-anywhere.md)) |

### state.db

Minimum tables, one model entry each: projects and project_repos ([Project config](../models/project-config.md#the-project-record)), [deployments](../models/deployment.md), [instances](../models/instance.md), [requests](../models/request.md), [team_messages](../models/team-message.md), [work_items](../models/work-item.md), [merge_requests](../models/merge-request.md), and [read_markers / ui_prefs](../models/ui-state.md). Schema changes ship as ordered migrations applied at startup; they were squashed into one `001_schema.sql` with the projects' tables (2026-10-20: databases from before are deleted once), and are squashed again before a release (the CEO, 2026-11-16). A migration never moves data into a file: what left the database for `projects.toml` (migration 027) moves with a one-off script ([Portable projects](../../backlog/portable-projects.md)). Driver and migration tooling: [Tech stack](./tech-stack.md).

- Resolved requests stay in the table as history.
- Removing a project deletes its rows in every table, in one transaction, and its session logs ([Add a project → Removing a project](../features/add-project.md#removing-a-project)): ids are built from names, so rows left behind would come back with a project of the same name.
- On restart, deployments and instances are restored from here ([Agent runtime](./agent-runtime.md#recovery)).

### Session logs

- The Terminal block appends every byte read from a session to its current log file before fanning it out ([Building blocks](./building-blocks.md#2-terminal-terminal)). A new session id (and file) per harness start.
- Scrollback for a newly attached terminal is replayed from the tail of the log (its last 1 MB, across the session's parts), so the UI shows history after a reload or a server restart; the server-side screen replaces it with [Terminal fidelity](../../backlog/terminal-fidelity.md).
- **Retention** (the CEO, 2026-10-20): each agent's folder, `logs/<agent>@<project>/`, stays under `log_cap_mb` (50 MB by default, a [global setting](../models/global-settings.md), read as logs are written). A session goes on in a new **part** each time its current one reaches a quarter of the cap (`<session-id>.log`, then `<session-id>.p000002.log` and on, so names sort oldest first); after each new part, and when a session starts, the oldest files go until the folder fits, older sessions before the session's own first parts, never the part being written. So up to about three quarters of the cap of the latest output is always kept, and the replayed tail reads across parts. Logs hold whatever the harness printed, minus the secrets Foreman redacts ([Security](./security.md#accepted-limitations-v1)).
- The activity line shown in [Project settings](../features/project-settings.md) comes from the harness's hooks and lives in the Instance manager's memory, never stored: a Foreman restart starts every instance without one, until its next hook ([Harness adapters → Activity lines](./harness-adapters.md#activity-lines)).

## File watching

- The home watches Foreman's HQ (`teams/`, `instructions.md`, `projects.toml`), `~/.foreman/settings.toml`, `projects/` (a project's HQ made or gone) and each project's HQ (`project.toml`). Code repos live anywhere and are never watched: they are checked on load, on every reload and before each start ([Project config](../models/project-config.md#the-project-record)); a repo found missing or changed is pushed as `projects.changed`. The wiki block watches every project's wiki bundle with a watcher of its own ([Wiki engine](./wiki-engine.md#what-foreman-adds-on-top)).
- Changes are debounced, reloaded and pushed to clients as events ([API](./api.md#events)) within ~1 s.
- A config file that fails to parse keeps its last good version in memory and surfaces the error (UI and server log) rather than breaking the project.
- Foreman's own writes to team files, the instructions, `projects.toml` and settings also produce watch events; they are recognised (by content version) so they don't bounce back as "external change".

## Team file writes

Team definitions are edited from the UI ([Agent page](../features/agent-page.md)) while the Chief of Staff or the CEO's editor may edit the same file. Writes are atomic (temp file + rename) and guarded by the content hash the UI last read, like wiki writes; a conflict is reported, never silently overwritten. A write re-encodes the whole TOML file, so comments the CEO wrote by hand don't survive it; each save is then **one commit** in Foreman's HQ, which keeps every earlier version; so is each instructions save. Settings are written the same way but not committed: `settings.toml` is outside git, and its history isn't kept ([Team definition](../models/team-definition.md#writes)).
