---
type: task
status: todo
priority: low
tags: [chore]
epic: terminal-multiplexer
---

# Import WikiShell's building blocks instead of copying them

**Why:** Foreman renders wikis as WikiShell does and holds copies of its code, because none of it can be imported ([Technical debt → Copies of WikiShell's building blocks](../technical-debt.md#copies-of-wikishells-building-blocks)). The CEO's decision (2026-10-17): copy now, upstream later.

**Done when:** Foreman imports what it copied today, and `internal/wiki/copied.go` and `ui/src/app/callout.ts` are gone; the technical-debt entry is removed.

- [x] the engine (`github.com/agentic-wiki/wiki`) exports its heading-id rule; Foreman drops its copy and the cross-check test (wiki v0.16.1, `Heading.ID`)
- [ ] WikiShell exports the view builders (entry, tree; the boards when Foreman has them) as a public Go package, and its renderer and callout plugin as a package the UI can install
- [ ] WikiShell gets a LICENSE
- [ ] Foreman imports them, at pinned versions
- [ ] report to WikiShell: a board over `/backlog` also takes `/backlogs/…`, since the engine's `Filter` matches a plain string prefix (Foreman passes the folder with its trailing slash; found 2026-10-17)
