---
type: spec
status: planned
tags: [architecture, wiki]
---

# Wiki engine

Every project has a **wiki bundle** at `<project>/wiki/`: Markdown files with YAML frontmatter plus a `wiki.toml`, compatible with the agentic-wiki / wikiview format (this repo's own `wiki/` is one). Foreman reads it for the [wiki reader](../features/wiki-reader.md) and the [board](../features/board.md), and makes exactly two kinds of writes: ticking a checkbox and moving a board card (rewriting one or two frontmatter fields). Agents (mainly the scrum master) and the CEO's editor do everything else.

## Decided: import the agentic-wiki engine

Foreman imports the Go module `github.com/agentic-wiki/wiki` ([Tech stack](./tech-stack.md)) and **does not reimplement any rule it owns**:

| Need | Engine API |
|---|---|
| Find and read the bundle config, `types`, `ignore` | `bundle.Discover`, `Bundle` fields |
| Board declarations `[[tool.wikiview.board]]` | `Bundle.DecodeTool("wikiview", &cfg)` |
| Tree, entries, frontmatter | `index.Build`, `Index.Filter`, `Index.Resolve`, `Entry.Frontmatter`/`Field`/`FieldList`, `Entry.Body` |
| Board membership (`where = ["type=task", …]`) | `index.ParseFilter` + `Index.Filter` (the same `key=value` / `key!=value` semantics as `wiki list --where`) |
| Links between entries (open in the reader), backlinks | `Entry.Links`, `Index.ResolveLink`, `Index.Backlinks` |
| Headings ("On this page"), checkboxes | `Entry.Headings`, `Entry.Checkboxes` (with line numbers) |
| Search | `Index.Search` |
| Card move: rewrite `status` (and lane field) | `Entry.SetField` / `SetFields`: rewrites only that line, preserves every other byte, writes atomically (temp file + rename) |
| Checkbox toggle | `Entry.SetCheckbox(line, done)`: exactly one character changes |

**Why this is a hard rule:** the engine's own retro on a reverted UI port found three implementations of link resolution and two frontmatter writers, each written where it was needed because the right home was unreachable. Foreman must not repeat that. If Foreman needs a rule the engine doesn't expose, the fix is an upstream contribution to the engine, not a local copy.

## What Foreman adds on top

- **Rendering** Markdown to what the reader shows: headings, paragraphs, lists, links between entries, callouts (`> [!warning]`, `[!success]`, `[!info]`, any word), tickable checkboxes, code blocks, inline images. The engine is deliberately not a Markdown renderer ([Tech stack](./tech-stack.md) decides where rendering happens).
- **Versions** for guarded writes: every entry served to the UI carries a version; a write with a stale version is rejected with a conflict ([API](./api.md#version-guarded-writes)).
- **Change tracking**: the file watcher ([Persistence](./persistence.md)) re-indexes changed bundles and pushes `wiki.changed` events within ~1 s; read markers ([UI state](../models/ui-state.md)) drive the "changed since you last looked" dots.
- **Board defaults** following wikiview: `where = ["type=task"]`, `status = "status"`; columns come from the declared `columns` (or the status vocabulary), lanes optionally from a field such as `priority`. Details in [Board](../features/board.md).

## Open questions

1. **Version-guarded writes.** `SetField`/`SetCheckbox` read the file at write time and apply the change, so they never clobber concurrent edits to *other* lines, but they can't tell that the UI acted on a stale view (e.g. the checkbox moved lines after an agent edit). Options: Foreman checks a version (content hash or mtime+size) right before calling the engine (small race window remains); or an upstream "write if unchanged since version X" API. Recommendation: hash check in Foreman now, propose the guarded API upstream.
2. **Incremental re-index.** `index.Build` scans the whole bundle. Rebuilding on every file event is fine for small wikis; large ones may need per-file refresh. Recommendation: debounce events (~200 ms) and rebuild the affected bundle; contribute incremental refresh upstream only if measurements demand it.
3. **Board config gaps.** Does the engine's wikiview config cover everything the board needs (`columns`, `lane`, `lanes`, `blockers`), and who validates it (unknown status values, missing lane values)? Recommendation: decode with the engine, validate in one place in Foreman, surface problems in the board UI.
4. **Engine version pinning.** The engine is pre-1.0 and its public API may change. Recommendation: pin a version, cover Foreman's uses with tests, upgrade deliberately.

The starter `wiki.toml` a new project needs is owned by [Add a project](../features/add-project.md#open-questions).
