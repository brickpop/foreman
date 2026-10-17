---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Wire the project pages to the real backend: Add a project and the Wiki block

**Why:** the project pages exist and say "Not built yet" for Add a project, Overview, Board and Changes ([Project pages](./ui-project-pages.md)); this card serves them from the real workspace and wikis ([Add a project](../product/features/add-project.md), [Wiki reader](../product/features/wiki-reader.md), [Board](../product/features/board.md), [Project settings](../product/features/project-settings.md), [Wiki engine](../product/architecture/wiki-engine.md)).

Was blocked by [Project pages](./ui-project-pages.md) and [Workspace and config](./workspace-config-events.md), both done (2026-10-16).

**Done when:** Add a project creates the project repo (`project/`: `git init`, `project.toml`, a starter wiki, first commit); Overview renders real entries (rendered as wikiview does, links through the engine; the changed dots are [their own card](./wiki-read-markers.md)); Board shows the boards from `wiki.toml` with version-guarded drags; Settings shows the real config; these areas leave "Not built yet" (Changes waits for epics 3–4).

- [x] candidates and Add (API), with an optional first empty code repo; New repo in Project settings (`POST /api/projects/:id/repos`); deploy refuses a project with no code repo (the CEO's decision, 2026-10-16: Foreman creates empty repos, never clones). Marking the new wiki's entries as seen waits for the UI state API
- [x] reading (2026-10-17): the Wiki block over the engine (pinned v0.10.0) serves the tree, entries (Markdown plus the engine's lookup tables) and the wiki's linked images; the reader renders with react-markdown as wikiview does ([Wiki engine → Rendering](../product/architecture/wiki-engine.md#rendering)); Overview and the side sheet read the real wiki
- [x] the wiki watcher (2026-10-17): follows each project's wiki, keeps its index, and sends `wiki.changed` only on a real change ([Wiki engine → Change tracking](../product/architecture/wiki-engine.md#what-foreman-adds-on-top), [API → Events](../product/architecture/api.md#events))
- [x] boards and writes (2026-10-17): boards laid out as wikiview does (the CEO's choice: undeclared statuses get their own columns, a No status column takes no drops); checkbox ticks and card moves guarded by the entry's version, one commit per write as the CEO, or written but left for the lead's next commit when the file has uncommitted changes (the CEO's choice); served from the real client ([Board](../product/features/board.md), [Sandboxes and git policy](../product/architecture/sandboxes-and-git.md#project-knowledge-base))
- [x] a project repo lacking the wiki's files (2026-10-17, found on the dev workspace's DevProject): Overview and Board show the [No wiki yet](../product/features/wiki-reader.md#no-wiki-yet) placeholder, whose button writes the missing scaffold files; Add a project uses the same scaffold
- [ ] the CEO tries it in the UI (`just dev`): read entries and images, tick a checkbox, move a card, see an agent's edit appear
- [ ] read markers: split out to [Wiki read markers](./wiki-read-markers.md) (the CEO, 2026-10-17: future, not a priority)
- [ ] known limits, tracked as debt: [Wiki performance](../technical-debt.md#wiki-performance-whole-wiki-work-on-every-change-and-read), [Wiki reading: known edges](../technical-debt.md#wiki-reading-known-edges)
- [ ] open: a link to another entry's `#anchor` opens that entry at its top (the anchor is dropped, as before); frontmatter values naming an entry aren't links yet (wikiview makes them links)
