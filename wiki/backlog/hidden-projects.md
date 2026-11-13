---
type: task
status: done
priority: medium
tags: [feature]
epic: workflow
---

# Hidden projects instead of archived

**Why:** the CEO, 2026-11-13: archiving has no real use for one person beyond putting a project out of sight. Today it also refuses deploys and gates Remove ([Project config → archived](../product/models/project-config.md#the-project-record)); refusing deploys protects the CEO from themselves, and Remove's own confirmation already says what goes.

Proposal: **hidden** is visibility only. The sidebar's collapsed group reads `More` instead of `Archived`; everything else works as for any project. Unreleased, so a plain rename: a migration renaming the column, `PUT /api/projects/:id/hidden`, no shim.

Decided (the CEO, 2026-11-13):

- [x] A hidden project with a team on it keeps running; folded, `More` shows one dot for its projects (needs-you first), so a waiting agent isn't missed
- [x] Remove: any project with no team deployed, behind its confirmation; no archive-first step
- [x] Dashboard: hidden ones left out, as in the sidebar
- [x] Migration 024 renames the column (an archived project stays out of sight); `PUT /api/projects/:id/hidden`; Deploy no longer refuses one; Project settings: a Visibility block in General, Remove disabled while a team is deployed
- [x] Tests: store (hide, the migration), home (hide with a team on it, remove without hiding), instances (a hidden project deploys), server route, UI (sidebar groups, `groupDot`, Dashboard rows, client)
