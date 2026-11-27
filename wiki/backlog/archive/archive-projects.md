---
type: task
status: done
priority: medium
tags: [feature, decision]
epic: projects-anywhere
---

# Archive and unarchive projects

**Why:** the sidebar lists every project ([App shell → Sidebar](../../product/features/app-shell.md#the-rail)), so finished or paused ones crowd out those in use. Until now the only way out was removing the project ([Project settings](../../product/features/project-settings.md)), which deletes its history ([Add a project, anywhere](./add-project-anywhere.md)). Raised by the CEO, 2026-10-23.

**Decided** (the CEO, 2026-10-23):

- **Archive**, not just hide: an archived project leaves the sidebar's projects and the Dashboard, and Deploy refuses it until it is unarchived. Its record, repos, worktrees, history and pages stay.
- **With teams deployed, archiving stops them first**, after a confirmation (the CEO, 2026-10-23, replacing an outright refusal); the server still refuses while one runs. Unarchiving is never refused.
- **Only an archived project can be removed** (the CEO, 2026-10-23): Remove says "Archive it first" until then.
- **The flag is on the project's record** ([Project config → archived](../../product/models/project-config.md#the-project-record)). The record is in this machine's database, so it is per machine, like the profile.
- **A collapsed "Archived" group (no count: the CEO, 2026-10-23)** sits after the project rows, before "+ New project"; Project settings archives and unarchives.
- **Only the sidebar and the Dashboard** leave it out. The setup checklist, the deploy dialog's project list and the gone-project check treat it as any project; the deploy dialog shows Deploy's refusal.

- [x] decisions above
- [x] the `archived` column (migration 008), `home.SetArchived`, `PUT /api/projects/{id}/archived`, Deploy's refusal
- [x] the sidebar's group, the Dashboard and Project settings follow
- [x] tests: store, home (refusal, event, restart), route, deploy, sidebar model, Dashboard rows, client
