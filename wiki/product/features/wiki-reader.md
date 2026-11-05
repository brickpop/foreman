---
type: spec
status: partial
tags: [feature, ui, wiki]
---

# Wiki reader

The project's **Wiki** section ([Project page](./project-page.md); Overview until 2026-10-31): a reader for the project's wiki bundle (`wiki/` in its project repo, `<workspace>/project/wiki/`), the team's shared memory. The content is the focus. There is no live status here; the wiki is content only (what waits on the CEO is in the [Inbox](./reviews.md#the-inbox)). Parsing, links and writes are done by the [Wiki engine](../architecture/wiki-engine.md).

## Header

Headed as a repo's page is ([Repos → A repo's page](./repos.md#a-repos-page); the CEO, 2026-11-02, so the two readers look alike): **`Knowledge base`** (18px, 600), and under it the wiki's folder on this machine, `<project repo>/wiki` (Mono 12.5px `--tx4`, ellipsis, its whole path as its title), the project repo being where it really is (`<workspace>/project`; for a project made before workspaces, `~/.foreman/projects/<name>`).

## The description

Under the header, the project's description (15px `--tx3`, at most 780px), from [Project config](../models/project-config.md), when it has one. The old project header (name, path, `<Team> · Sessions` links, tabs) is gone since 2026-10-31: the name and sections are in the [sidebar](./app-shell.md#sidebar), the work in [Session](./sessions.md), the path in [Settings](./project-settings.md) (`<path>/project.toml · read-only`).

## Layout

A grid (gap 24) using the full width, as a repo's page: the file tree, **always shown** (the CEO, 2026-11-01), `minmax(180px, 260px)`, then the entry, `minmax(0, 1fr)`, at most 820px wide. The tree and the entry **scroll apart**, each in its own pane, the page filling the window (the CEO, 2026-11-02): opening an entry shows its top while the tree stays where it is. Under 760px the tree goes above the entry, behind the `Files` toggle. There the two stack and the page scrolls as one. Body padding `24px 24px 24px` (`48px` at the bottom under 760px), 16px between the header, the description and the grid.

## Toolbar

- **`Files`** toggle, **under 760px only** (above it the tree is always shown, so no toggle): a file-tree icon on its left (raised when open), **closed by default**. It carries no changed dot: those sit on the entries and folders in the tree.

## File tree

Sticky (top 16px). The whole bundle at any depth: in each folder, `index.md` first, then subfolders, then entries, alphabetically. Folders are collapsed except those on the path to the current entry. A click anywhere on a folder's row expands or collapses it; expanding a folder that has an `index.md` also opens it.

- **The file tree's look**, the same as a repo's ([Repos → A repo's page](./repos.md#a-repos-page)): a folder row has a chevron (turned down when open) and a folder icon (open or closed), a file row a file icon (both `--tx4`), in line with the folder names; an open folder's contents are indented 13px under its chevron, with a 1px `--line` guide down their left. Rows padding `3px 6px`, 13.5px, `--tx2`, ellipsis on overflow; the entries by their title. The current entry: `--accSoft`, `--accTx`. (The CEO, 2026-11-02, replacing indentation by padding alone.)
- A 6px attention (`--attn`) **changed dot** on entries changed since last viewed, and on a collapsed folder holding one ([UI state](../models/ui-state.md#schema) defines "seen").

## Article

As a repo's file is shown: the entry's path in the wiki (Mono 12.5px `--tx2`, ellipsis) stays at the top of the view while it scrolls, then the entry in a panel (padding `20px 24px`), gap 14 inside. Opens `index.md` by default.

No "On this page" list (the CEO, 2026-10-17: removed; a heading map at the side, as wikiview has, only if it ever proves needed). Headings still take the engine's ids, so `#anchor` links scroll to them, and a link to another entry's heading (`x.md#heading`, from the reader or a board card's sheet) opens that entry scrolled to it (the URL's `#heading`, so Back and a shared link keep it; 2026-11-05). Reloading the entry (a tick) doesn't scroll again.

1. **[Frontmatter](#frontmatter)**, folded to one line at first. A `title` field isn't among it: it is the page's H1, above the body (the CEO, 2026-10-21: a card's body doesn't repeat it). An entry with both a `title` and a body H1 shows both.
2. **Body** rendered from Markdown:

| Element | Rendering |
|---|---|
| H1 | Page title (24px; the CEO, 2026-11-02: 30px was too big) |
| H2 | 18px / 600, anchor target |
| H3 | 15.5px / 600, anchor target (new: the prototype rendered no H3) |
| Paragraph | 15.5px / 1.65 / `--tx2`, with inline emphasis, code and links |
| List item | `–` bullet in `--tx4` in an 18px gutter |
| Link to another entry | Opens it in the reader (`--accTx`), resolved by the engine ([Wiki engine → Rendering](../architecture/wiki-engine.md#rendering)); a path written percent-encoded (`my%20plan.md`) names the file as it is named (`my plan.md`; the CEO, 2026-11-03). External links open in a new tab |
| Link to an entry not written yet | `--tx3`, dotted underline, titled "Not written yet"; still opens (and falls back to `index.md`) |
| Link above the wiki (e.g. into a code repo) | Text, `--tx3` with a dotted underline, titled `<href> is outside the wiki`: never followed |
| Link to a file the wiki carries | An image opens in a new tab; any other file (a PDF, a spreadsheet) is text like a link above the wiki, titled `Foreman doesn't show <path>`: Foreman serves the wiki's images only |
| Callout `> [!warning] Title`, `[!success]`, `[!info]`, any word | `--panel`, radius 10, padding `12px 16px`, a colored border and an uppercase label (an element, so a screen reader hears it): warning `--attn`/`--attnTx`, success `--ok`, info `--info`, any other `--line3`/`--tx3`. The label is the title, else the type word |
| Checkbox `- [ ]` | Tickable (see below) |
| Code block | `<pre>` on `--term`, radius 8 |
| Image | Inline, max width of the column, resolved by the engine. Only the wiki's own images load (the CEO, 2026-10-17); an image elsewhere shows as a link to it, and a missing one as its alt text, dotted |
| Table | Bordered cells (`--line2`), scrolling on its own when wider than the column |
| Quote | A `--line3` left rule, `--tx3` |
| Raw HTML | Shown as text, never rendered: agents write these files |

## Frontmatter

As wikiview shows it (its `EntryView`; the CEO, 2026-11-02, replacing the `key: value` chips): a panel above the body (`--panel`, `--line` border, radius 8, 12.5px), the same everywhere a Markdown file shows, in the reader, the board's card sheet and a repo's file ([Repos](./repos.md#a-repos-page)).

- **Folded** (at first): a chevron, then one line of the values without their keys, split by ` · ` and clipped, never wrapped. Empty fields are left out; a field naming several entries shows their count (`2 blocked_by`).
- **Open**: `Properties`, then a grid of keys (`--tx4`, never colored: color is for what can be clicked) and values. An empty field says `(nothing)`; `tags` are chips, each with its colored dot (one color per tag, from its name, the same everywhere); any other list is a chip per value; a scalar is text, clipped with its whole value as its tooltip.
- **A value naming an entry** (a reference, as the engine reads one: it ends in `.md`, relative to the entry or from the wiki's root with a leading `/`, and an entry exists there) shows that entry's title, underlined in `--accTx`, and opens it; the value as written is its tooltip. One climbing above the wiki, or naming no entry, is plain text. A repo's file names no entries: its values are all text.
- **Open or folded** is remembered in the browser (not per entry, not on the server): it opens as the CEO last left it. Not built from wikiview: the folded line floating the grid over the text on hover.
- **Not shown**: the root `index.md`'s `okf_version` (the format's bookkeeping, hidden as in wikiview; the CEO, 2026-10-17) and the engine's own keys (a leading `_`). The fields are in key order: the engine keeps no file order.

## Behaviour

- **Opening an entry** (tree, link, board "Open in wiki"): shows it in the Wiki section (`?entry=<path>`), closes the side sheet, marks the entry seen, scrolls the entry's pane to its top (the tree keeps its place). An unknown path falls back to `index.md`.
- **Ticking a checkbox** writes that one character to the file atomically, guarded by the entry's version; on conflict the box reverts and an error toast explains it ([API](../architecture/api.md), `PUT /api/wiki/:project/checkbox/:path`).
- **File changes** (agents or the CEO's editor) appear within about 1 s ([Persistence](../architecture/persistence.md)): the open entry re-renders, and changed entries get their dot.
- Prose is read-only. The only writes are a checkbox tick here and a card move on the [Board](./board.md).

## No wiki yet

A project repo can lack the files the reader and the [Board](./board.md) need: one whose files were deleted, or a cloned one that never had them ([Add a project → Effect](./add-project.md#effect)). Then the **Wiki and Board** sections show, instead of the reader or the board (the tree is irrelevant without a wiki; the CEO, 2026-10-17), a dashed placeholder: `This project's wiki isn't set up yet.`, `Foreman reads it once its project repo has:` and each missing file (Mono, `<project>/project/<file>`), then a primary **`Create them`** (`Create it` for one). It writes each missing file of the [scaffold](./add-project.md#effect), never over one that exists, and commits them in the project repo as the CEO (`Project repo: add …`); the wiki appears as the watcher picks it up. `GET` / `POST /api/projects/:id/scaffold` ([API](../architecture/api.md)). Any other failure to read the wiki shows its message, never a blank page.

Only the files the wiki can't be read without (`wiki/wiki.toml`, `wiki/index.md`) bring up the placeholder. The rest of the scaffold (`AGENTS.md`, `.gitignore`, `product/index.md`, `backlog/index.md`, `technical-debt.md`) leaves the wiki and the board in place, with a one-line **notice** above them (the CEO, 2026-10-17): each missing file (Mono) and what goes wrong without it (`AGENTS.md is missing: the agents don't know how this project's wiki and tasks work.`), and a **`Create it`** / **`Create them`** button doing the same as the placeholder's. Each missing file's reason comes from the server, with whether it is required.
