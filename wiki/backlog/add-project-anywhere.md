---
type: task
status: todo
priority: high
tags: [feature]
epic: projects-anywhere
---

# Add a project, anywhere

**Why:** the New project dialog lists a workspace folder's subfolders; projects now take repos from anywhere ([Projects anywhere](./epics/projects-anywhere.md)).

**Done when:**
- **New project:** a name, then its project repo: **New** (an empty one, starter wiki) or **Clone** (a git URL); then its repos, picked from disk (a folder browser listing the git repos under `$HOME`, and a path field; outside `$HOME` refused), each named after its folder unless renamed. A new empty code repo, as today, in a folder the CEO picks.
- **Project settings → Repos:** add, rename, locate a missing one, remove one (refused while it's deployed? to decide with the card).
- **Remove a project** (Project settings): refused while deployed; after a confirmation naming what goes (its record, its sandboxes), with **also delete its project repo folder** as an option, off by default; code repos keep their branches.
- The sidebar, Home and the setup checklist follow (no workspace tree).
- Tests: the dialog's helpers, the routes and their refusals.

- [ ] the dialog; the folder browser (`$HOME` only, behind the access token)
- [ ] Project settings' repos; removing a project
- [ ] specs: Add a project, Project settings, Settings, App shell, First run
