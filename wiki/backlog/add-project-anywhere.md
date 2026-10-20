---
type: task
status: todo
priority: high
tags: [feature]
epic: projects-anywhere
---

# Add a project, anywhere

**Why:** projects are records whose repos live anywhere in the CEO's home folder ([Projects as records](./projects-as-records.md), [Projects anywhere](./epics/projects-anywhere.md)). That card built the routes and removed the UI that called the old ones; this card brings the UI back on the new routes.

**What exists (from [Projects as records](./projects-as-records.md)):** `POST /api/projects` {name} (the record and its project repo, New, starter wiki); `POST /api/projects/{id}/repos` {name, path, new?} (an existing repo, or a new empty one at a path that doesn't exist yet); `PUT /api/projects/{id}/repos/{repo}` {path} (Locate); `DELETE /api/projects/{id}/repos/{repo}` (409 DEPLOYED while a team is deployed on the project); `PUT /api/projects/{id}/profile` {profile}. Errors: 400 INVALID, 409 EXISTS, 409 DEPLOYED. The UI follows `projects.changed`.

**Done when:**
- **The New project dialog comes back**, with the sidebar's **+ New project** entry: a name, then its project repo: **New** (an empty one, starter wiki) or **Clone** (a git URL, and its credentials question); then its repos, picked from disk (a folder browser listing the git repos under `$HOME`, and a path field; outside `$HOME` refused), each named after its folder unless renamed, or a **new empty code repo** at a path the CEO picks.
- **The setup checklist's "project" and "repo" steps get their action buttons back** (New project; add or create a repo on the project).
- **Project settings → Repos manages the repos:** add an existing one, create a new empty one, **Locate** a missing or changed one, remove one (refused while a team is deployed on the project, as the route does). Renaming a repo has no route yet: to decide with the card (its name is in the agents' sandbox paths).
- **Remove a project** (Project settings): refused while deployed; after a confirmation naming what goes (its record, its sandboxes), with **also delete its project repo folder** as an option, off by default; code repos keep their branches. Needs its route (`DELETE /api/projects/{id}`).
- The sidebar, Home and the setup checklist follow (projects from the records).
- Tests: the dialog's helpers, the clone and remove routes and their refusals.

- [ ] the dialog and the sidebar entry; the folder browser (`$HOME` only, behind the access token); Clone
- [ ] the checklist's project and repo actions
- [ ] Project settings' repos (add, new, locate, remove); removing a project
- [ ] specs: Add a project, Project settings, Settings, App shell, First run
