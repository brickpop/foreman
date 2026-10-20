---
type: task
status: todo
priority: high
tags: [feature]
epic: projects-anywhere
---

# Projects as records

**Why:** a project becomes a chosen set of repos anywhere on disk, with its project repo under `~/.foreman/projects/<name>/` ([Projects anywhere](./epics/projects-anywhere.md)).

**Done when:**
- The database holds the projects (name, created) and their repos (a name unique in the project, an absolute path), and each project's profile (from the settings split, once decided).
- Each project's `project.toml` (in its project repo) keeps only its rules: `main`, `checks`, `always_ask_ceo`, `forge`, `recursive_submodules`; `repos` and `project` leave it (the name is the record's).
- Loading checks each repo (exists, is a git repo, inside `$HOME`, its real path) and each project repo; a missing one is a problem shown on the project, and blocks starting its agents, not the rest.
- **Locate** points a missing repo at a new folder, accepted only if it is the same repo (a commit both have); **remove** takes a repo out of the project.
- The file watcher follows the project repos and the global repo; the repo folders are checked on load and on each start, not watched (they're anywhere).
- The API serves projects from the database; `workspace.changed` becomes `projects.changed`.
- Tests: the records, missing and located repos, a repo in two projects, the watcher.

- [ ] the records and the API
- [ ] `project.toml` slimmed, its checks
- [ ] missing, locate, remove
- [ ] specs: Workspace, Project config, Persistence, API
