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

Several boards can sit over one folder (e.g. "Sprint 07" by priority, and "Findings" with `where = ["type=task", "kind=finding"]`).

## Layout

Body padding `20px 40px 64px`.

- **Toolbar**: the **board switcher** (segmented, one button per declared board; on = raised with `--line3` border) and, right-aligned, a Mono **`wiki.toml`** / **`Hide wiki.toml`** button that reveals the board's declaration in a `<pre>` (max 620px), quoted from the file.
- **Grid**, min width 880px, scrolling horizontally when narrower.
  - Header row: one cell per column, `<Label> <count>` (label 500 `--tx3`, count `--tx4`). Labels come from the column values (`todo` → "To do", `in-progress` → "In progress", `review` → "Review", `done` → "Done"; others title-cased).
  - **Lanes**: with a `lane` field, one row per lane value, labelled (12px `--tx4`) e.g. "High priority". Without one, a single unlabelled row.
  - **Cell**: `--side`, `--line` border, radius 10, padding 8, min height 72px, gap 8.
  - **Card**: `--panel`, `--line` border, radius 8, padding `10px 12px`, grab cursor; hover border `--line3`. Top row: the id (the file's basename, Mono 500 11.5px `--tx4`) and the priority (colored, [Design system](../design/design-system.md#chips-and-badges)); the title (14px, line-height 1.4); Avatar 20 and the assignee's name (12.5px `--tx3`) when the entry has an `assignee` that matches an agent.
- Footer note: `Drag a card to change its status (and its lane). Only those lines of the task's frontmatter are rewritten.`

## Interactions

- **Drag** a card to another cell: sets the column field to the column value and, on a board with lanes, the lane field to the lane value. Only those frontmatter lines change. The write is guarded by the entry's version: on conflict the card snaps back and an error toast says the file changed. A drop target cell is highlighted while dragging. Success shows a toast ([App shell](./app-shell.md#toasts)). API: `PUT /api/wiki/:project/card/:board/:path` ([API](../architecture/api.md)).
- **Click** a card: opens the **side sheet**.

## Side sheet

Fixed right, `min(460px, 100vw)` ([App shell](./app-shell.md#overlays)).

- Header: `<project>/wiki/<path>` (Mono 12.5px `--tx3`), **`Open in wiki`** (small outline; opens the entry in the [Wiki reader](./wiki-reader.md)) and `×`.
- Body (padding `20px 22px 32px`, gap 12): frontmatter chips, then the rendered entry at sheet sizes (H1 22px, H2 15px, paragraphs 14.5px). Checkboxes are tickable, exactly as in the reader.

## Open questions

1. **Where columns and lanes come from.** The prototype hard-codes the four columns and the high/medium/low lanes. Options: from `columns`/`lanes` in `wiki.toml` (wikiview keys, as this repo's own `wiki.toml` uses), falling back to the values found in the entries, in first-seen order; always fixed. Recommendation: `wiki.toml` first, then the values found.
2. **Cards with a missing or unknown status or lane value.** They vanish in the prototype. Options: hide; show them in an extra "Other" column/lane. Recommendation: an "Other" column/lane at the end, so no entry is ever invisible.
3. **A board with no kind filter.** In the prototype, the sprint board shows findings mixed with tasks because it only filters `type=task`. Is that intended? Recommendation: yes, boards show exactly what their `where` says; the fix belongs in `wiki.toml`, not in the UI.
4. **Lane drags rewrite two fields.** PRD §10.4 says a drag "rewrites only that frontmatter line", and the prototype's footer says the same, but a drop into another lane also changes the lane field (e.g. `priority`); the API is `PUT …/card/:board/:path {value, lane, version}` ([API](../architecture/api.md)). Confirm that a lane drag rewrites both lines (as described above) in one guarded write.
