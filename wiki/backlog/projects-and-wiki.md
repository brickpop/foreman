---
type: task
status: todo
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Wire the project pages to the real backend: Add a project and the Wiki block

**Why:** the project pages exist and say "Not built yet" for Add a project, Overview, Board and Changes ([Project pages](./ui-project-pages.md)); this card serves them from the real workspace and wikis ([Add a project](../product/features/add-project.md), [Wiki reader](../product/features/wiki-reader.md), [Board](../product/features/board.md), [Project settings](../product/features/project-settings.md), [Wiki engine](../product/architecture/wiki-engine.md)).

Was blocked by [Project pages](./ui-project-pages.md) and [Workspace and config](./workspace-config-events.md), both done (2026-10-16).

**Done when:** Add a project creates the project repo (`project/`: `git init`, `project.toml`, a starter wiki, first commit); Overview renders real entries (server-side markdown, links through the engine, changed dots); Board shows the boards from `wiki.toml` with version-guarded drags; Settings shows the real config; these areas leave "Not built yet" (Changes waits for epics 3–4).

- [x] candidates and Add (API), with an optional first empty code repo; New repo in Project settings (`POST /api/projects/:id/repos`); deploy refuses a project with no code repo (the CEO's decision, 2026-10-16: Foreman creates empty repos, never clones). Marking the new wiki's entries as seen waits for the UI state API
- [ ] the Wiki block over the agentic-wiki engine; goldmark rendering, raw HTML off
- [ ] the workspace watcher follows each project's wiki bundle; `wiki.changed` events ([API → Events](../product/architecture/api.md#events))
- [ ] entries, boards, checkbox and field writes, each committed in the project repo (one commit per write, only that file, authored as the CEO), read markers (API + events)
- [ ] serve the areas from the real client (`ui/src/api/client.ts`), so they leave "Not built yet"
