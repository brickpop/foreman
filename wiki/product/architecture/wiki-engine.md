---
type: spec
status: partial
tags: [architecture, wiki]
---

# Wiki engine

Every project has a **wiki bundle**, its knowledge base, at the root of the [project's HQ](../models/project-config.md#the-projects-hq) (`<folder>/projects/<project>/hq/`): Markdown files with YAML frontmatter plus a `wiki.toml`, compatible with the agentic-wiki / WikiShell format (this repo's own `wiki/` is one). Foreman reads it for the [wiki reader](../features/wiki-reader.md) and the [board](../features/board.md), and makes three kinds of writes: ticking a checkbox, moving a board card (rewriting one or two frontmatter fields), and, once per wiki, renaming the old board tables in `wiki.toml` (below). Each is **committed** in the project repo, one commit per write, holding only that file, authored as the CEO. An agent's [`contribute`](./protocol.md#contribute) is not one of them: it is a draft in the project's shared files until the lead files it (the CEO, 2026-11-18) ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)). The lead (committing through `commit()`) and the CEO's editor do everything else.

## Decided: import the agentic-wiki engine

Foreman imports the Go module `github.com/agentic-wiki/wiki` ([Tech stack](./tech-stack.md)) and **does not reimplement any rule it owns**:

| Need | Engine API |
|---|---|
| Find and read the bundle config, `types`, `ignore` | `bundle.Discover`, `Bundle` fields |
| Board declarations `[[board]]` | `Bundle.DecodeBoards(&boards)`, safe from many requests at once (the engine holds the lock) |
| Tree, entries, frontmatter | `index.Build`, `Index.Filter`, `Index.Resolve`, `Entry.Frontmatter`/`Field`/`FieldList`, `Entry.Body` |
| Board membership (`where = ["type=task", …]`) | `index.ParseFilter` + `Index.Filter` (the same `key=value` / `key!=value` semantics as `wiki list --where`) |
| Links between entries (open in the reader), backlinks | `Entry.Links`, `Index.Backlinks`, `index.Read` (a repo's Markdown) |
| Frontmatter values that name an entry, titles | `Entry.FrontmatterRefs`, `Entry.Title` |
| Headings ("On this page"), checkboxes | `Entry.Headings`, `Entry.Checkboxes` (with line numbers) |
| Search | `Index.Search` |
| Card move: rewrite `status` (and lane field) | `Entry.SetField` / `SetFields`: rewrites only that line, preserves every other byte, writes atomically (temp file + rename) |
| Checkbox toggle | `Entry.SetCheckbox(line, done)`: exactly one character changes |

**Why this is a hard rule:** the engine's own retro on a reverted UI port found three implementations of link resolution and two frontmatter writers, each written where it was needed because the right home was unreachable. Foreman must not repeat that. If Foreman needs a rule the engine doesn't expose, the fix is an upstream contribution to the engine, not a local copy.

## What Foreman adds on top

- **Rendering**: the engine is deliberately not a Markdown renderer. The browser renders, from the engine's answers sent as data ([below](#rendering)).
- **Versions** for guarded writes: every entry served to the UI carries a version (a content hash); right before calling `SetField` or `SetCheckbox`, Foreman re-hashes the file and rejects a stale write with a conflict ([API](./api.md#version-guarded-writes)). The engine's writes already never clobber concurrent edits to *other* lines; the hash adds "the UI acted on what is on disk". A small race window remains between the check and the write; a "write if unchanged since version X" API is to be proposed upstream rather than built here. Writes run one at a time, and each rebuilds the wiki's index before answering, so a refetch reads it (the watcher still sends `wiki.changed` for it). Built 2026-10-17.
- **Change tracking** (built 2026-10-17, after WikiShell's watcher and store): the wiki block's own file watcher ([Persistence](./persistence.md)) follows every project's wiki, recursively, skipping the folders the engine's own `Build` never enters (`Index.SkipsFolder`: hidden ones, and those `wiki.toml`'s `ignore` covers), re-applied when `wiki.toml` changes. It also watches the folder holding the wiki, only so a wiki made later is seen: nothing else created there (a project's `files/`) is followed. A folder that moves or goes drops its watches, and so does a removed project, with the folder holding its wiki. On macOS every watched folder and file holds a file descriptor, so watching more than the wiki can exhaust the process's limit. It follows new projects and forgets removed ones as the home reports them. It debounces events (~200 ms) on entries, `wiki.toml` and images, rebuilds the affected wiki's index with `index.Build` (incremental refresh is contributed upstream only if measurements ever demand it), keeps it for every read until the next rebuild, compares each entry's content hash, and pushes `wiki.changed` ([API](./api.md#events)) only on a real change: a save of the same bytes, a swap file or a file the wiki doesn't serve sends nothing. Without the watcher (tests), each read builds a fresh index. Read markers ([UI state](../models/ui-state.md)) drive the "changed since you last looked" dots.
- **Board defaults** following WikiShell: `where = ["type=task"]`, `status = "status"`; columns come from the declared `columns` (or the status vocabulary), lanes optionally from a field such as `priority`. The board declaration is decoded by the engine (`Bundle.DecodeBoards`) and validated in one place in Foreman; problems (unknown keys, missing values) surface in the board UI. Details in [Board](../features/board.md).
- **Renaming the old board tables** (the ADR "Boards and graphs in the standard", 2026-11-19; until Foreman's first release on or after 2027-01-01, [Technical debt](../../technical-debt.md#the-wikiview-board-rename)): before it builds a wiki's index, under the lock every write takes, Foreman renames `[[tool.wikiview.board]]` and `[[tool.wikiview.graph]]` headers in `wiki.toml` to `[[board]]` and `[[graph]]`, and deletes a bare `[tool.wikiview]` header whose table is empty. Every other byte stays (comments, order, line endings); a file that already has `[[board]]` tables is renamed too, since arrays of tables join. It saves the file atomically (a hidden temporary file, then a rename; never through a symlink), then commits it alone, as the CEO, with the message `Rename [tool.wikiview] tables in wiki.toml to [[board]] and [[graph]]`, unless `wiki.toml` already has uncommitted changes: then the rename goes with the lead's next commit. A second read finds nothing to change. What it can't rename, and a save that fails, are board problems ([Board](../features/board.md)).
- **Version pinning**: the engine is pre-1.0, so Foreman pins a version (v0.19.0, and the agent image's `wiki` CLI the same), covers every use it makes of the engine with tests, and upgrades deliberately.
- **Unreadable frontmatter**: an entry whose frontmatter isn't valid YAML, or is over one of the engine's limits, has no fields. Foreman shows the engine's reason (`frontmatterError` on the entry, its tree node and a repo's Markdown file), lists such entries under each board's path in `unread` instead of as cards, and answers a write the engine refuses with 422 `REFUSED` and the reason. A repo's Markdown file is read by the engine's rules too (`index.Read`).

## Rendering

Decided by the CEO (2026-10-17): Foreman renders wikis the way [WikiShell](https://github.com/agentic-wiki/wiki-shell) does, reusing its proven building blocks, rather than with goldmark on the server.

- **The server sends an entry's Markdown exactly as on disk** (frontmatter stripped), plus **lookup tables** for every rule the browser must not work out itself (`GET /api/wiki/:project/entry/:path`, [API](./api.md)):
  - **links**: each link's href as written (the lookup key), resolved by the engine (`Entry.Links`, `Entry.Outside`): the wiki path, its anchor, whether the entry exists (a link may point at an entry not written yet, shown dotted), whether it is a file the wiki carries and whether that file is an image the reader shows, or whether it climbs above the wiki (shown as text, never followed).
  - **headings**: each one's id, by the engine's rule, so an `#anchor` that `wiki check` accepts always scrolls.
  - **checkboxes**: each one's line in the file (what `SetCheckbox` is addressed by) and in the body (what the renderer matches).
  - **frontmatter**: the fields by key, each value as written and, when it names an entry, that entry's path and title ([Wiki reader](../features/wiki-reader.md#frontmatter)).
  - Headings and checkboxes are matched by **source line**, never by counting nodes as they render.
- **The browser renders** with `react-markdown`, `remark-gfm` and a callout plugin (`> [!warning]`, `[!success]`, `[!info]`, any word, its label an element a screen reader hears). **Raw HTML is off**: it shows as text. Script URLs are dropped.
- **Only the wiki's own images load** (the CEO, 2026-10-17): an image an entry links to, inside the wiki once symlinks are followed, of an image type (PNG, JPEG, GIF, WebP, AVIF, SVG), served by `GET /api/wiki/:project/image/:path` with `nosniff`, and an SVG under a policy that forbids script. An image elsewhere would make the browser call any address an agent writes, so it shows as a link.
- **Titles**: an entry's `title` field, else its body's first `# heading` (outside code fences; the CEO, 2026-10-21: a wiki that titles its entries by heading gets titled cards too), else its file name made readable (`003-watch-and-events.md` reads "003 Watch and events"), the same in the tree, the article and backlinks.

**Copied, for now:** WikiShell's Go code is internal and its UI isn't a package, so Foreman holds copies of the readable-name rule, the callout plugin and the frontmatter panel ([Wiki reader](../features/wiki-reader.md#frontmatter)), each marked with its origin. Heading ids, link destinations (percent-decoding included), frontmatter references and titles come from the engine (`Heading.ID`, `Link.Target`, `Entry.FrontmatterRefs`, `Entry.Title`). The copies go when WikiShell and the engine export them ([Technical debt](../../technical-debt.md)).
