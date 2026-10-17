---
type: task
status: todo
priority: low
tags: [chore]
epic: terminal-multiplexer
---

# Import wikiview's building blocks instead of copying them

**Why:** Foreman renders wikis as wikiview does and holds copies of its code, because none of it can be imported ([Technical debt → Copies of wikiview's building blocks](../technical-debt.md#copies-of-wikiviews-building-blocks)). The CEO's decision (2026-10-17): copy now, upstream later.

**Done when:** Foreman imports what it copied today, and `internal/wiki/copied.go` and `ui/src/app/callout.ts` are gone; the technical-debt entry is removed.

- [ ] the engine (`github.com/agentic-wiki/wiki`) exports its heading-id rule; Foreman drops its copy and the cross-check test
- [ ] wikiview exports the view builders (entry, tree; the boards when Foreman has them) as a public Go package, and its renderer and callout plugin as a package the UI can install
- [ ] wikiview gets a LICENSE
- [ ] Foreman imports them, at pinned versions
