---
type: task
status: done
priority: high
tags: [feature]
epic: projects-anywhere
---

# Add a project, anywhere

**Why:** projects are records whose repos live anywhere in the CEO's home folder ([Projects as records](./projects-as-records.md), [Projects anywhere](../epics/projects-anywhere.md)). That card built the routes and removed the UI that called the old ones; this card brings the UI back on the new routes.

## Decided (the CEO, 2026-10-20)

- **Clone** runs the CEO's own git on the host: their SSH keys, agent and credential helper. Foreman stores nothing; a failure shows git's message. A local path clones too (a copy; the folder is never moved in).
- **The folder browser** goes folder by folder (one folder's subfolders at a time, hidden ones hidden), marks git repos (their top folder) with an Add button, and has a path field beside it. It starts at the folder `foreman` was started in, or `$HOME` when that is outside `$HOME`, and never offers `..` above it.
- **That start folder bounds adding a repo** (browser, pasted path, Locate, a new empty repo), as well as `$HOME`: a path outside it is refused with the reason. Repos already added stay usable wherever they are.
- **Renaming a repo:** refused while a team is deployed on the project (like Remove); renames the record and moves each agent's existing worktree (`git worktree move`), so stopped deployments keep their work. Merge requests keep the old name in their history.
- **Removing a project deletes its history** (its deployments, instances, messages, actions, work items, merge requests, requests and session logs), so re-adding the name starts clean: ids are built from the name (`atlas@ledger`), and a redeploy would otherwise revive the old rows. This replaces the epic's "its history stays in the database".
- **Renaming a repo moves its open merge requests** to the new name, in the same transaction; closed ones keep the old name.
- **A gone project undeploys** (folded in from *Stop a deployment whose project folder was deleted*): when a check finds every code repo missing, or the project repo folder gone, it checks again 60 s later (a move or a git operation in progress); still gone, its deployments **Stop**, with the reason on the deployment and a toast. One repo of several missing only blocks starts, as now.

**Done when:**
- **The New project dialog comes back**, with the sidebar's **+ New project** entry: a name, then its project repo: **New** (an empty one, starter wiki) or **Clone** (a git URL or a local path, cloned with the CEO's own git); then its repos, picked from disk (the folder browser, or a pasted path; bounded as decided above), each named after its folder unless renamed, or a **new empty code repo** at a path the CEO picks.
- **The setup checklist's "project" and "repo" steps get their action buttons back** (New project; add or create a repo on the project).
- **Project settings → Repos manages the repos:** add an existing one, create a new empty one, **Locate** a missing or changed one, remove one (refused while a team is deployed on the project, as the route does). Rename one (as decided above).
- **Remove a project** (Project settings): refused while deployed; after a confirmation naming what goes (its record, its sandboxes), with **also delete its project repo folder** as an option, off by default; code repos keep their branches. Needs its route (`DELETE /api/projects/{id}`).
- The sidebar, the Dashboard and the setup checklist follow (projects from the records).
- **A gone project undeploys** as decided above.
- Tests: the dialog's helpers, the browser's bounds, the clone, rename and remove routes and their refusals, the undeploy after the recheck.

**Left for later:** component tests of the dialog and the picker ([UI component tests](../ui-component-tests.md)).

- [x] the dialog and the sidebar entry; the folder browser (bounded by the start folder, behind the access token); Clone
- [x] the checklist's project and repo actions
- [x] Project settings' repos (add, new, locate, rename, remove); removing a project
- [x] a gone project undeploys after a recheck
- [x] specs: Add a project, Project settings, App shell, First run, Deployment (lifecycle), API
