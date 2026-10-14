---
type: spec
status: planned
tags: [architecture, wiki]
---

# Wiki engine

Every project has a **wiki bundle**, its knowledge base, at `<project>/project/wiki/` in the [project repo](../models/project-config.md#the-project-repo): Markdown files with YAML frontmatter plus a `wiki.toml`, compatible with the agentic-wiki / wikiview format (this repo's own `wiki/` is one). Foreman reads it for the [wiki reader](../features/wiki-reader.md) and the [board](../features/board.md), and makes exactly three kinds of writes: ticking a checkbox, moving a board card (rewriting one or two frontmatter fields), and filing an agent's [`contribute`](./protocol.md#contribute) note in `inbox/`. Each is **committed** in the project repo, one commit per write, holding only that file: a contribution authored as the agent, a tick or card move as the CEO ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)). The lead (committing through `commit()`) and the CEO's editor do everything else.

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

- **Rendering** Markdown to what the reader shows: headings, paragraphs, lists, links between entries, callouts (`> [!warning]`, `[!success]`, `[!info]`, any word), tickable checkboxes, code blocks, inline images. The engine is deliberately not a Markdown renderer: Foreman renders on the server with goldmark, raw HTML disabled, links resolved by the engine's own resolver, checkboxes carrying their source line ([Tech stack](./tech-stack.md)).
- **Versions** for guarded writes: every entry served to the UI carries a version (a content hash); right before calling `SetField` or `SetCheckbox`, Foreman re-hashes the file and rejects a stale write with a conflict ([API](./api.md#version-guarded-writes)). The engine's writes already never clobber concurrent edits to *other* lines; the hash adds "the UI acted on what is on disk". A small race window remains between the check and the write; a "write if unchanged since version X" API is to be proposed upstream rather than built here.
- **Change tracking**: the file watcher ([Persistence](./persistence.md)) debounces events (~200 ms) and rebuilds the affected bundle's index with `index.Build` (incremental refresh is contributed upstream only if measurements ever demand it), and pushes `wiki.changed` events within ~1 s; read markers ([UI state](../models/ui-state.md)) drive the "changed since you last looked" dots.
- **Board defaults** following wikiview: `where = ["type=task"]`, `status = "status"`; columns come from the declared `columns` (or the status vocabulary), lanes optionally from a field such as `priority`. The board declaration is decoded by the engine (`bundle.DecodeTool`) and validated in one place in Foreman; problems (unknown keys, missing values) surface in the board UI. Details in [Board](../features/board.md).
- **Version pinning**: the engine is pre-1.0, so Foreman pins a version, covers every use it makes of the engine with tests, and upgrades deliberately.
