---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Wiki read markers: the "changed since you last looked" dots

**Why:** the reader's tree is meant to show which entries changed since the CEO last looked ([Wiki reader](../product/features/wiki-reader.md), [UI state](../product/models/ui-state.md)). The wiki is served and followed live (built 2026-10-17), but nothing records what the CEO has seen, so every node's `changed` is false. Split out of the Wiki block by the CEO (2026-10-17): in the future, not a priority.

**Done when:** opening an entry marks it seen; the tree's dots show entries changed since, and collapsed folders holding one; a new project's starter entries start seen ([Add a project](../product/features/add-project.md), step 6); tests cover each.

- [ ] the read markers in `state.db`, keyed by project and entry, holding the version seen ([UI state](../product/models/ui-state.md))
- [ ] reading an entry marks it seen; the tree fills `changed`; `wiki.changed` refreshes it
- [ ] Add a project marks the starter wiki seen
