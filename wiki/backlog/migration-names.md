---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Catch a migration whose file changed after it ran

**Why:** the CEO, 2026-11-13. Migrations are numbered only: `state.db` keeps `PRAGMA user_version`, never which file ran ([Persistence](../product/architecture/persistence.md)). On 2026-11-12 a database had run an interim `022_asked_target.sql`; the committed `022_landing.sql` took its number, so it never ran there, and the server failed later on `no such column: landing`, far from the cause. Foreman is unreleased, so this only bites a database that ran uncommitted work, but then it bites silently.

Proposal (to settle when the card starts): record each applied migration's name and a hash of its text in a table, and at startup refuse, saying which, when a file at an applied number has a different name or text. Not a rerun, not a repair: the message tells the CEO what diverged.

- [ ] Decide: name only, or name and hash (a hash also catches an edited file, but a whitespace fix would then need a way through)
- [ ] Record applied migrations; check them at startup, before migrating
- [ ] What a database from before the table does: trust its `user_version` and backfill from the files
- [ ] Tests: a renamed and an edited migration at an applied number both refuse, naming the file; a fresh and an up-to-date database start
- [ ] Persistence spec

Related: [Versioning](./versioning.md) (what an older Foreman does with a newer database).
