---
type: task
status: todo
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# State store: SQLite with migrations

**Why:** deployments, instances, messages, requests and work items all live in `state.db` ([Persistence](../product/architecture/persistence.md)); the Instance manager needs deployments and instances first.

**Done when:** `.foreman/state.db` is opened (WAL mode) with numbered embedded migrations applied through `PRAGMA user_version`, each component owning its own tables behind its own interface, tested on a real SQLite in a temp dir.

- [ ] `store` package: open, migrate, transactions; `modernc.org/sqlite` ([Tech stack](../product/architecture/tech-stack.md#server-go))
- [ ] deployments and instances tables ([Deployment](../product/models/deployment.md), [Instance](../product/models/instance.md))
- [ ] tests: migrations from empty, from every earlier version, concurrent readers
