---
type: task
status: done
priority: medium
tags: [feature]
epic: workflow
---

# A folder's index.md as a row of the wiki tree

**Asked (the CEO, 2026-11-05):** a folder's `index.md` opened only when expanding the folder, so with the folder already open it took two clicks, and nothing showed it was there. Decided: the index is the folder's first row, named `<title> - index.md`; the folder row only expands and collapses ([Wiki reader → File tree](../../product/features/wiki-reader.md#file-tree)).

- [x] the tree's `Node` carries a folder's `index.md` as its first child; `hasIndex` removed
- [x] `treeLabel`: `<title> - index.md` for any `index.md`, the root's included
- [x] the folder row toggles only
