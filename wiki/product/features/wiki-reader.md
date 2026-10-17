---
type: spec
status: partial
tags: [feature, ui, wiki]
---

# Wiki reader

The project's **Overview** tab: a reader for the project's wiki bundle (`<project>/project/wiki/`), the team's shared memory. The content is the focus. There is no live status here; the wiki is content only. Parsing, links and writes are done by the [Wiki engine](../architecture/wiki-engine.md).

## Project header and tabs

Shared by every project tab:

- Header: the project name (H1), its description (15px `--tx3`) and, right-aligned and bottom-aligned, the folder path (Mono 12.5px `--tx4`, e.g. `~/agents-ws/ledger`). From [Project config](../models/project-config.md).
- Tabs (padding `9px 14px`, 500): `Overview · Changes · Board · Settings`. Active: `--tx` with a 2px `--acc` bottom border over the 1px `--line` rule; inactive `--tx3`. **Changes** carries a count badge of the project's open requests ([Changes](./changes.md)). Switching tabs closes the side sheet. Every tab's body has the same sides as the header (48px) and uses the full width, so the four tabs line up.

## Layout

A grid (gap 32) using the full width. Tree closed: the article alone, `minmax(0, 1fr)`. Tree open: `minmax(150px, 200px) minmax(0, 1fr)`, so the article shrinks by the tree. Body padding `24px 48px 64px`.

## Toolbar

- **`Files`** toggle, a file-tree icon on its left (raised when open). **Closed by default.** It carries no changed dot: those sit on the entries and folders in the tree.
- The current entry path (Mono 12px `--tx4`, ellipsis): `<project>/project/wiki/<path>`.

## File tree

Sticky (top 16px). The whole bundle at any depth: in each folder, `index.md` first, then subfolders, then entries, alphabetically. Folders are collapsed except those on the path to the current entry. A click anywhere on a folder's row expands or collapses it; expanding a folder that has an `index.md` also opens it.

- Indentation: `10 + 14 × depth` px left padding; padding `5px 10px`; 13.5px; ellipsis on overflow.
- Folders: 500, `--tx2`, led by a chevron (`--tx4`, turned down when open). Entries: 400, `--tx3`, their text in line with the folder names. The current entry: `--raised`, `--tx`.
- A 6px amber **changed dot** on entries changed since last viewed, and on a collapsed folder holding one ([UI state](../models/ui-state.md#schema) defines "seen").

## Article

Full width of its column, gap 14. Opens `index.md` by default.

1. **"On this page"** (12px `--tx4` label): the entry's H2 and H3 headings (13px `--tx3`, H3 indented 12px), each smooth-scrolling to its heading (24px offset). Shown only when there are any; bottom border.
2. **Frontmatter chips**: one `key: value` chip per field; empty values hidden, and so is the root `index.md`'s `okf_version` (the format's bookkeeping, hidden as in wikiview; the CEO, 2026-10-17).
3. **Body** rendered from Markdown:

| Element | Rendering |
|---|---|
| H1 | Page title (30px) |
| H2 | 18px / 600, anchor target |
| H3 | 15.5px / 600, anchor target (new: the prototype rendered no H3) |
| Paragraph | 15.5px / 1.65 / `--tx2`, with inline emphasis, code and links |
| List item | `–` bullet in `--tx4` in an 18px gutter |
| Link to another entry | Opens it in the reader (`--accTx`), resolved by the engine ([Wiki engine → Rendering](../architecture/wiki-engine.md#rendering)). External links open in a new tab |
| Link to an entry not written yet | `--tx3`, dotted underline, titled "Not written yet"; still opens (and falls back to `index.md`) |
| Link above the wiki (e.g. into a code repo) | Text, `--tx3` with a dotted underline, titled `<href> is outside the wiki`: never followed |
| Link to a file the wiki carries | An image opens in a new tab; any other file (a PDF, a spreadsheet) is text like a link above the wiki, titled `Foreman doesn't show <path>`: Foreman serves the wiki's images only |
| Callout `> [!warning] Title`, `[!success]`, `[!info]`, any word | `--panel`, radius 10, padding `12px 16px`, a colored border and an uppercase label (an element, so a screen reader hears it): warning `--acc`/`--accTx`, success `--ok`, info `--info`, any other `--line3`/`--tx3`. The label is the title, else the type word |
| Checkbox `- [ ]` | Tickable (see below) |
| Code block | `<pre>` on `--term`, radius 8 |
| Image | Inline, max width of the column, resolved by the engine. Only the wiki's own images load (the CEO, 2026-10-17); an image elsewhere shows as a link to it, and a missing one as its alt text, dotted |
| Table | Bordered cells (`--line2`), scrolling on its own when wider than the column |
| Quote | A `--line3` left rule, `--tx3` |
| Raw HTML | Shown as text, never rendered: agents write these files |

## Behaviour

- **Opening an entry** (tree, link, board "Open in wiki"): shows it, switches to Overview, closes the side sheet, marks the entry seen, scrolls to the top. An unknown path falls back to `index.md`.
- **Ticking a checkbox** writes that one character to the file atomically, guarded by the entry's version; on conflict the box reverts and an error toast explains it ([API](../architecture/api.md), `PUT /api/wiki/:project/checkbox/:path`).
- **File changes** (agents or the CEO's editor) appear within about 1 s ([Persistence](../architecture/persistence.md)): the open entry re-renders, and changed entries get their dot.
- Prose is read-only. The only writes are a checkbox tick here and a card move on the [Board](./board.md).
