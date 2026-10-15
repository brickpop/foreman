---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: full-ui-demo
---

# Project pages and the Add a project dialog

**Why:** where a project's knowledge and progress live ([Wiki reader](../product/features/wiki-reader.md), [Changes](../product/features/changes.md), [Board](../product/features/board.md), [Project settings](../product/features/project-settings.md), [Add a project](../product/features/add-project.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** the project header and tabs; Overview renders wiki entries (headings, lists, links between entries, callouts, checkboxes, code, images; "On this page"; frontmatter chips; the Files tree with changed dots); Changes shows the request panels with Merge to main, Create PR and Reject, In flight and Merged; Board shows boards, columns and lanes with drag and drop and the side sheet; Settings shows teams on the project and the read-only config; the Add a project dialog lists candidates.

- [x] project header (name, description, folder) and tabs (Changes badge)
- [x] Overview: the shared `WikiArticle` ("On this page", frontmatter chips, server HTML with callouts, code, lists; links open entries, external ones a new tab; checkboxes tick, version-guarded), the Files tree (index first, folders on the path open, changed dots on entries and collapsed folders), an unknown path falls back to index.md, reading marks seen
- [x] Changes: needs you (the shared request panel in its Changes variant: kind chip first, source, link line, Merge to main / Create PR / Open PR, Reject with its reason), in flight (work items, merge requests with what they carry, stopped deployments), merged (last 7 days)
- [x] Board: switcher, `wiki.toml`, columns and lanes with counts and Other, native drag and drop shown at once and put back on failure, a status menu per card, the side sheet with Open in wiki and ticks, toasts
- [x] Settings: teams on the project (needs `GET /api/deployments`, now built) and the read-only configuration (with the file as on disk: `Project.source`); the Add a project dialog (candidates, `+ New project`)
- Not yet: a member's last terminal line (needs the `instance.line` event); Sync (Git epic); the Deploy dialog (Team and Agent card)
- [ ] the CEO looks at it on the Mac (`just dev`)
