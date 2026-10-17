---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Wiki reader: anchors into other entries, and frontmatter links

**Why:** two gaps left when the Wiki block shipped (2026-10-17), both from wikiview's reader, which has them ([Wiki reader](../product/features/wiki-reader.md)):

- A link to another entry's `#anchor` opens that entry at its top: the anchor is dropped. It should scroll to the heading, whose id the server already sends.
- Frontmatter values that name an entry (`epic: /backlog/epics/x.md`, `blockers: [...]`) show as plain text. wikiview makes any `.md` value that resolves to an entry a link, resolved by the engine (`ResolveLink`), never by the browser.

**Done when:** both work in the reader and the board's side sheet; tests cover them.

- [ ] cross-entry anchors: open the entry, then scroll to the heading
- [ ] frontmatter references: the server sends them resolved (as wikiview's `frontmatterRefs`), the chips link them
