---
type: task
status: todo
priority: medium
tags: [decision]
epic: workflow
---

# Hidden projects instead of archived

**Why:** the CEO, 2026-11-13: archiving has no real use for one person beyond putting a project out of sight. Today it also refuses deploys and gates Remove ([Project config → archived](../product/models/project-config.md#the-project-record)); refusing deploys protects the CEO from themselves, and Remove's own confirmation already says what goes.

Proposal: **hidden** is visibility only. The sidebar's collapsed group reads `More` instead of `Archived`; everything else works as for any project. Unreleased, so a plain rename: a migration renaming the column, `PUT /api/projects/:id/hidden`, no shim.

Open (proposed answers in brackets, waiting on the CEO):

- [ ] A hidden project with a team on it: [keeps running; `More` shows its attention dot, so a waiting agent isn't missed]
- [ ] Remove: [allowed on any project with no team deployed, behind its confirmation; no archive-first step]
- [ ] Dashboard: [hidden ones left out, as in the sidebar]
- [ ] Then: store, home, server, UI (sidebar, Dashboard, Project settings, Deploy dialog), specs, tests
