---
type: task
status: done
priority: medium
tags: [feature]
epic: install-first-run
---

# Rename a project: its display name

**Why:** the CEO, 2026-11-04: "I want to be able to rename a project".

**Decided (the CEO, 2026-11-04):** a **display name only**. The id (the slug) is in instance ids, every agent branch (`wt/<id>/…`, `lead/<id>/…`), its workspace folder and Foreman's log and conversation folders; renaming all of that is a big, risky move for little gain. The display name is how it shows, everywhere a person reads it ([Project config → The project record](../../product/models/project-config.md#the-project-record)).

**Built (2026-11-04):**

- [x] `projects.name` (migration 017), `Home.SetName` (trimmed, one line, at most 80 characters, "" clears; allowed while a team runs), `PUT /api/projects/:id/name`
- [x] the UI: `projectName` (the name, else the readable id) wherever a project shows (sidebar, Dashboard, titles, toasts, notifications, dialogs); the **Name** block in [Project settings](../../product/features/project-settings.md#configuration)
- [x] tests: `TestSetName`, the route's, `projectName`, the sidebar's and a notification's

- [x] (round 2, the CEO) names unique, in any case, against each project's shown name (`Project.Shown`, the server's `readable` as the UI's): refused by the server (`SetName`, `NewProject`), warned in Project settings (`nameTakenBy`); `TestProjectNamesAreUnique`, `TestReadable`
- [x] (round 2, the CEO) the sidebar, Dashboard and Deploy dialog sort projects by name (`byName`), replacing latest first: `GET /api/activity/latest`, `Activity.Latest`, `latestFirst` and `useLatestActivity` removed
- [x] (round 2) the server refuses a code repo in Foreman's own folders (`outsideForemans`: add, create, locate), as the browser never opens them; `TestNoCodeRepoInForemansFolders`
- [x] (round 2) `git.Switch` starts a new branch from `refs/heads/<base>`: a tag of the CEO's named like the base (`main`) made git refuse ("ambiguous object name"), so no agent could start a branch there; `TestSwitchCreatesFromMainThenComesBack`
