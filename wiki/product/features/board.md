---
type: spec
status: planned
tags: [feature, ui, wiki]
---

# Board

The project's **Board** tab: a kanban over wiki entries, compatible with wikiview. Boards are declared in the project's `wiki/wiki.toml` under `[[tool.wikiview.board]]`; cards are entries (by default `type: task`), columns come from a status field, and optional lanes from another field. Moving a card rewrites one frontmatter line, so agents see the change through the file. Parsing and writes: [Wiki engine](../architecture/wiki-engine.md).

## Board declaration

```toml
[[tool.wikiview.board]]
id      = "sprint"
path    = "/backlog"          # folder the cards come from
name    = "Sprint 07"
where   = ["type=task"]       # filters, AND-ed (default ["type=task"])
status  = "status"            # column field (default "status")
columns = ["todo", "in-progress", "review", "done"]
lane    = "priority"          # optional lane field
lanes   = ["high", "medium", "low"]
```

Several boards can sit over one folder (e.g. "Sprint 07" by priority, and "Findings" with `where = ["type=task", "kind=finding"]`). A board shows exactly what its `where` selects: a sprint board filtering only `type=task` also shows findings, and the fix is a `kind` filter in `wiki.toml`, not special-casing in the UI.

**Columns and lanes** come from `columns` and `lanes` in `wiki.toml` when declared; otherwise from the values found in the matching entries, in first-seen order. An entry whose status (or lane value) is missing or not in the list goes to an **Other** column (or lane) at the end, so no entry is ever invisible. Validation of the declaration lives in one place ([Wiki engine](../architecture/wiki-engine.md)).

## Layout

Body padding `24px 48px 64px`, like every project tab.

- **Toolbar**: the **board switcher** (segmented, one button per declared board; on = raised with `--line3` border) and, right-aligned, a Mono **`wiki.toml`** / **`Hide wiki.toml`** button that reveals the board's declaration in a `<pre>` (max 620px), quoted from the file.
- **Grid**, min width 880px, scrolling horizontally when narrower.
  - Header row: one cell per column, `<Label> <count>` (label 500 `--tx3`, count `--tx4`). Labels come from the column values (`todo` → "To do", `in-progress` → "In progress", `review` → "Review", `done` → "Done"; others title-cased).
  - **Lanes**: with a `lane` field, one row per lane value, labelled (12px `--tx4`) e.g. "High priority". Without one, a single unlabelled row.
  - **Cell**: `--side`, `--line` border, radius 10, padding 8, min height 72px, gap 8.
  - **Card**: `--panel`, `--line` border, radius 8, padding `10px 12px`, grab cursor; hover border `--line3`. Top row: the id (the file's basename, Mono 500 11.5px `--tx4`) and the priority (colored, [Design system](../design/design-system.md#chips-and-badges)); the title (14px, line-height 1.4); Avatar 20 and the assignee's name (12.5px `--tx3`) when the entry has an `assignee` that matches an agent.
- Footer note: `Drag a card to change its status (and its lane). Only those lines of the task's frontmatter are rewritten.`

## Interactions

- **Drag** a card to another cell: sets the column field to the column value and, on a board with lanes, the lane field to the lane value. Only those frontmatter lines change, both in one write (a drop into another lane rewrites two lines, unlike the PRD's "only that line"). The write is guarded by the entry's version: on conflict the card snaps back and an error toast says the file changed. A drop target cell is highlighted while dragging. Success shows a toast ([App shell](./app-shell.md#toasts)). API: `PUT /api/wiki/:project/card/:board/:path` ([API](../architecture/api.md)).
- **Status menu**: every card has a small menu (keyboard-reachable, and the way to move cards on touch screens) listing the columns (and lanes); choosing one does exactly what a drag does. Drag and drop is native HTML5 ([Tech stack](../architecture/tech-stack.md)).
- **Click** a card: opens the **side sheet**.

## Side sheet

Fixed right, `min(460px, 100vw)` ([App shell](./app-shell.md#overlays)).

- Header: `<project>/project/wiki/<path>` (Mono 12.5px `--tx3`), **`Open in wiki`** (small outline; opens the entry in the [Wiki reader](./wiki-reader.md)) and `×`.
- Body (padding `20px 22px 32px`, gap 12): frontmatter chips, then the rendered entry at sheet sizes (H1 22px, H2 15px, paragraphs 14.5px). Checkboxes are tickable, exactly as in the reader.
