---
type: task
status: blocked
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Wire the project pages to the real backend: Add a project and the Wiki block

**Why:** the project pages exist on the demo client ([Project pages](./ui-project-pages.md)); this card serves them from the real workspace and wikis ([Add a project](../product/features/add-project.md), [Wiki reader](../product/features/wiki-reader.md), [Board](../product/features/board.md), [Project settings](../product/features/project-settings.md), [Wiki engine](../product/architecture/wiki-engine.md)).

Blocked by [Project pages](./ui-project-pages.md) and [Workspace and config](./workspace-config-events.md).

**Done when:** Add a project creates the project repo (`project/`: `git init`, `project.toml`, a starter wiki, first commit); Overview renders real entries (server-side markdown, links through the engine, changed dots); Board shows the boards from `wiki.toml` with version-guarded drags; Settings shows the real config; these areas leave the demo client (Changes stays on demo until epics 3–4).

- [ ] candidates and Add (API)
- [ ] the Wiki block over the agentic-wiki engine; goldmark rendering, raw HTML off
- [ ] the workspace watcher follows each project's wiki bundle; `wiki.changed` events ([API → Events](../product/architecture/api.md#events))
- [ ] entries, boards, checkbox and field writes, each committed in the project repo (one commit per write, only that file, authored as the CEO), read markers (API + events)
- [ ] swap the areas in main.tsx; remove their badges
