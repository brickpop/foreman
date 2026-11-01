---
type: task
status: done
priority: medium
tags: [feature, ui]
epic: full-ui-demo
---

# No avatars; readable project names

**Why:** "team member avatars.... this is just useless. Can you please completely remove the feature? the face selector thing, fields, etc. it only makes things ugly, and complex for no reason", and "Actual project folder name should be slug case but printed name should be desluggified. Also in cards and similar elsewhere." (the CEO, 2026-11-02).

## Decided (the CEO, 2026-11-02)

- **Avatars go completely:** no faces, no Look section, no `look` in team files. A status dot stands in where the avatar carried a state (Session tabs, Deployment rows); elsewhere the agent's name is enough.
- **Old team files still load:** their `look` is ignored and dropped on the next save ([Technical debt → Old team files' `look`](../../technical-debt.md#old-team-files-look)).
- **A new project's name is a slug** (`my-portfolio`), its id and workspace folder; the New project dialog makes it from whatever is typed. Existing projects keep their names.
- **Project names show readable** everywhere a person reads them ("My portfolio"): [Project config → The project record](../../product/models/project-config.md#the-project-record).

## Done

- [x] Go: `Soul.Look` and its validation gone; `decode` skips retired keys (`retiredLook`); `cos.Create(ctx, id)` and `POST /api/cos/proposal {id}`; the CoS prompt says to leave an old `look` out, and its schema refuses one
- [x] Go: `NewProject` and `SuggestWorkspace` take slug names only (`validProjectName`); repos keep their looser rule
- [x] UI: `Avatar` deleted; `statusColor`/`dotStyle` in `ui.tsx`; the Agent page's Look section, the CoS's faces, `randomLook` and the palettes gone; status dots on Session tabs and Deployment rows; the team card lists its members' names
- [x] UI: `readable(name)` (was `repoTitle`) for every project name shown: sidebar, team rows, Dashboard cards, notifications, toasts, dialogs, headers, empty states, the Agent page; `projectSlug` in the New project dialog, with its hint
- [x] Tests: an old `look` loads and is dropped on save, a misplaced one still errors; slug project names (and the refused ones); `readable`, `projectSlug`, `statusColor`, the sidebar's team rows, notifications and reviews naming projects readably
- [x] Specs: Agent soul, Team definition, Agent page, Chief of Staff, API, Dashboard, Team page, Reviews, Sessions, Project deployment, Project pipeline, Board, Design system, Tech stack, Protocol, Glossary, Journey, Use cases, Project config, Add a project, App shell; the Avatar entry and its prototype deleted
