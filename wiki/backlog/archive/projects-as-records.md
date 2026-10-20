---
type: task
status: done
priority: high
tags: [feature]
epic: projects-anywhere
---

# Projects as records, sandboxes under the home

**Why:** a project becomes a chosen set of repos anywhere on disk, with its project repo under `~/.foreman/projects/<name>/`, and agents' worktrees leave the CEO's folders for `~/.foreman/sandboxes/<project>/<agent>/` ([Projects anywhere](../epics/projects-anywhere.md)). One card since 2026-10-20 (the CEO): with no project folder there is nowhere for `.sandboxes/`, so neither half runs without the other.

## Decided (the CEO, 2026-10-20)

- **No workspace any more:** `foreman` takes no folder; the home holds the global repo (`config/`: `teams/`, `settings.toml`, its `.gitignore`; nothing else under the home is in a repo), `state.db`, the access token, `credentials/`, `logs/`, `cos/`, `projects/`, `sandboxes/`. Fresh start: the CEO copies their teams by hand.
- **The Go package `workspace` becomes `home`** (`home.Home`; the UI's generated `workspace.ts` becomes `home.ts`), as the last, mechanical step.
- **The migrations are squashed into one `001_schema.sql`**, with the projects' tables: dev databases are deleted once.
- **The same team on two projects that share a repo:** the deploy is refused, naming the repo and the other project (their branches, `wt/<agent>/*` and `lead/<team>/*`, would collide in that repo's `.git`). Different teams never collide: agent ids are unique.
- **Code only older installs needed goes:** `adoptLogins`/`profiles.Adopt`, team files' `harness`/`command`, adopting unlabelled processes, the review backfill of merges made before reviews. The global repo's `.gitignore` top-up stays (every new ignored line needs it).
- **Clone a project repo** (and its credentials question) comes with [Add a project, anywhere](./add-project-anywhere.md), with the dialog. This card builds **New** (starter wiki).

## The design

- **Records** (`store.Projects()`): `projects` (id, profile, created_at) and `project_repos` (project, name, path, root commit), names unique per project in any case. The project's profile moves here from `settings.toml`'s `project_profiles` (gone), set with `PUT /api/projects/{id}/profile`.
- **A repo** is added by an absolute path that is a git repo's top folder (a `.git` folder, since it is mounted read-only at its own path), with at least one commit, its real path inside the real `$HOME`, outside the home, not twice in a project. Its root commit is kept, so **Locate** accepts only a folder holding it. Existing worktrees follow a located repo (`git worktree repair` on the next start). A missing or changed repo is a problem on the project and blocks starting its agents (deploy, start, restart), not the rest.
- **`project.toml`** keeps only rules: `description`, `main`, `checks`, `merge_to_main`, `always_ask_ceo`, `recursive_submodules`, `forge`; `project` and `repos` go.
- **Routes** (for the dialog of the next card): `POST /api/projects` {name} (the project repo, New), `POST /api/projects/{id}/repos` {name, path, new?} (an existing repo, or a new empty one at a path that doesn't exist yet), `PUT /api/projects/{id}/repos/{repo}` {path} (Locate), `DELETE /api/projects/{id}/repos/{repo}` (refused while a team is deployed on the project), `PUT /api/projects/{id}/profile`. `GET /api/candidates` goes, `GET /api/workspace` becomes `GET /api/home` ({path}); `workspace.changed` becomes `projects.changed`. Removing a project comes with the next card.
- **Watching:** the global repo (`config/settings.toml`, `config/teams/`), `projects/` and each project repo's `project.toml`. Repo folders are checked on load, on every reload and on each start, never watched.
- **Sandboxes** at `<home>/sandboxes/<project>/<agent>/<repo>`; mounts: the sandbox, each repo's `.git` read-only at its own path, the project repo at `<sandbox>/project`. No `.git/info/exclude` any more. Processes are labelled with the home (`foreman.home`), not a workspace.
- **The prompt** names no project folder (`({path})` goes), which settles [container writes](../container-writes-outside-sandbox.md)'s prompt cause.
- **UI, kept to what must change:** the generated types, the profile picker's route, paths shown (a project repo, a team file under the home), `projects.changed`. The New project dialog and Project settings' new-repo form go until the next card (the routes they called are gone).
- **`just dev`:** its home is `.dev/<harness>` itself; it seeds `config/teams/`, `config/settings.toml` and the DevProject repos as today, and the project record through the API once the server answers.

## Steps

- [x] store: squashed schema, projects and repos, tests
- [x] the home: records-driven projects, repo checks, add/new/locate/remove, profile, watcher, `project.toml` and settings slimmed, legacy gone; tests
- [x] consumers: instances (sandboxes, mounts, label, the shared-repo refusal, blocked starts), workflow, merges, repos browser, prompt, Chief of Staff, server routes and events, `cmd/foreman` (no folder argument)
- [x] UI: types, client, profile, paths, the dialog and new-repo form removed
- [x] `just dev`, README
- [x] rename `workspace` → `home`
- [x] specs: Workspace (the home), Project config, Workspace settings, Persistence, API, Sandboxes and git policy, Session runner, Session prompt, Security, Glossary, Add a project, Project settings

**Left open:** the known edges (a stopped deployment holding its repos, Locate to a clone, a rescan on every start) are in [Technical debt](../../technical-debt.md#projects-as-records-known-edges). To finish: the CEO runs `just dev` and `just dev fake` on the Mac after deleting `.dev/real` and `.dev/fake`.
