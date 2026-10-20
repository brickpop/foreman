---
type: task
status: done
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# State store: SQLite with migrations

**Why:** deployments, instances, messages, requests and work items all live in `state.db` ([Persistence](../../product/architecture/persistence.md)); the Instance manager needs deployments and instances first.

**Done when:** `.foreman/state.db` is opened (WAL mode) with numbered embedded migrations applied through `PRAGMA user_version`, each component owning its own tables behind its own interface, tested on a real SQLite in a temp dir.

- [x] `store` package: open, migrate, transactions; `modernc.org/sqlite` ([Tech stack](../../product/architecture/tech-stack.md#server-go))
- [x] deployments and instances tables ([Deployment](../../product/models/deployment.md), [Instance](../../product/models/instance.md))
- [x] tests: migrations from empty, from every earlier version, concurrent readers (plus: a failed migration rolls back, gaps are refused, a newer database is refused, concurrent deploys of one pair, workspace paths with `?`, `#` and `%`)
