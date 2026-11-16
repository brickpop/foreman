---
type: task
status: done
priority: high
tags: [feature]
epic: install-first-run
---

# Project workspaces: a project's files in a folder of the CEO's

**Why:** the CEO, 2026-10-28. A project's repo and its agents' worktrees live hidden under `~/.foreman` (`projects/<name>/`, `sandboxes/<name>/<agent>/`), next to Foreman's private data (token, logins, state.db). The CEO wants to find them, and to go into an agent's worktree and run the code it is touching. So each project gets a **workspace**, a folder of the CEO's, and `~/.foreman` keeps only Foreman's own data. Found while discussing [Restructure the New project dialog](../new-project-dialog.md), which builds on this card.

## Decided (the CEO, 2026-10-28)

```
~/workspace/foreman/              the projects folder (setting projects_folder)
└─ ledger/                        the project's workspace
   ├─ project/                    the project repo (project.toml, AGENTS.md, wiki/): always here
   └─ agents/
      ├─ mara/                    mara's sandbox: a worktree of each code repo, on her branch
      │  └─ ledger-api/
      └─ tove/ …
```

- **The workspace** is chosen at New project, pre-filled `<projects folder>/<name>` and editable; the record keeps its path. The **project repo is always `<workspace>/project/`**, made there (New) or cloned there (Clone). No "existing project repo elsewhere".
- **The projects folder**: a global setting `projects_folder`, kept as a `~` path in `settings.toml` so it carries to other machines. Default `~/workspace/foreman` when that folder exists; when it doesn't, New project offers to create it, or to pick another.
- **Code repos stay where they are**, outside the workspace: the CEO's own checkouts. A repo may be in several projects (a docs repo several projects read). The workspace holds the agents' worktrees of them.
- **Bounds: anywhere in `$HOME`, never inside `~/.foreman`**, for workspaces and code repos alike. This **replaces the start-folder bound** (2026-10-20, [Add a project → Picking repos](../../product/features/add-project.md#picking-repos)): the folder browser opens at the folder Foreman was started from, as a convenience, and can go up to `$HOME`.
- **The harness's own files leave the sandbox**: `.harness/` (the private config, conversation transcripts, account record, the login's mount point) moves to `~/.foreman/harness/<project>/<agent>/`, which stays readable only by the CEO (0700), whereas a workspace isn't (`ConfigDir` in each adapter).
- **Existing projects: offered a migration**, per project, while nothing is deployed on it: the project repo and every sandbox (stopped deployments' worktrees too) move into the workspace, `git worktree repair` in each code repo, `.harness/` to its new place, and **each agent's conversations carried over**: Claude Code and Pi file a conversation under its working folder's path, so each adapter renames its own folder to the new path, and agents continue where they were.
- **Removing a project** keeps the workspace (the CEO's files), and offers to clean up the agents' worktrees (on a project kept, the Team section's [Clean up](../../product/features/project-team.md#earlier-teams) does it, only what is safe).

## To keep in mind (told to the CEO)

- **Running an agent's code on the Mac**: agents install dependencies in a Linux container, so native ones (`node_modules`, wheels) don't run on macOS, and the CEO's own install breaks the agent's. Fine on Linux. The other routes: [Reach a server an agent runs in its container](../container-ports.md), or checking the branch out in the CEO's own repo.
- **Editing in an agent's worktree** while it works turns into the agent's uncommitted changes: the UI says to look and run, not edit, or to stop the agent first.
- An agent's absolute sandbox path is in its record (`inst.SandboxPath`), its prompt and its worktrees' links: everything that holds it changes with a migration.
- The watcher follows each project's own folder instead of `~/.foreman/projects/`.
- Direct engine: a workspace inside an enclosing git repo (`~/workspace` a repo itself) is visible to a process run directly on the host; in a container it isn't mounted.

## Built (2026-10-28)

- **The record** has a `workspace` (migration `009`); `Home.layout` derives the project repo (`<ws>/project`), the sandboxes (`<ws>/agents`) and the harness folders (`~/.foreman/harness/<project>`) from it, or the old paths when it is empty (legacy). `Project.Sandbox(agent)`, `Project.HarnessDir(agent)`.
- **The takeover stays, redefined:** a git repo already at `<workspace>/project` is taken over, as one in `~/.foreman/projects/<name>` was (no name-case rule any more: the place is the workspace, not the name).
- **Bounds:** `Home.usable` (inside `$HOME`, not inside Foreman's home) for workspaces, repos and the browser, which opens at the start folder and goes up to `$HOME`; `checkWorkspace` adds: not a file, not inside a code repo, no overlap with another project's.
- **Harness:** `harness.Session.Dir`, the instance's harness folder, mounted at the same path (0700); each adapter's config dir is `Dir/<harness>`. `harness.Mover` (Claude Code: `projects/<path with non-alphanumerics as ->` plus its `.claude.json` entry; Pi 1.3.0: `sessions/--<path>--`), both read from the harnesses' own code; wired into `home.Deps.Conversations` in `main.go`.
- **Migrate** (`POST /api/projects/{id}/migrate`): refusals, the moves, `git worktree repair` per repo, conversations, undone in reverse on a failure.
- **Remove:** keeps the workspace; `deleteWorktrees` and `deleteRepo` options; harness folders and logs go.
- **UI:** New project's Workspace field (`GET /api/folders/workspace`), Project settings' Workspace block (Copy) or Move offer, Remove's worktrees checkbox, Settings → System → Projects folder.
- **Tests:** home (layout, rules, takeover, remove, browse bounds, migrate: worktrees work and survive a prune, uncommitted work kept, conversations hook, harness folders moved, refusals, a failure puts everything back), harness (each mover, the rollback across adapters), instances (the harness folder mounted, 0700), server routes, UI client.
- Known edges: [Technical debt → Foreman's folder](../../technical-debt.md#foremans-folder-known-edges) (the workspaces and their migration went with [One Foreman folder](../foreman-folder.md)).

## Steps

- [x] specs: Home, Project config (The workspace), Sandboxes and git policy, Add a project, Global settings, Harness adapters (Moving conversations), Session runner, Project settings, Settings, API, and the paths elsewhere
- [x] the record's workspace; paths derived from it; the harness folders under the home
- [x] `projects_folder` and its default; the `$HOME` bound for every path, replacing the start folder
- [x] the watcher on each workspace's project repo
- [x] migration, undone if a step fails (temporary: [One Foreman folder](../foreman-folder.md))
- [x] remove: keep the workspace, offer the worktrees' clean-up
- [x] tests
- [ ] the CEO: create a project in a workspace and deploy on it (Podman); move an existing project with a stopped deployment, then deploy again: the agents continue their conversations (Claude Code, and Pi if used) and their worktrees hold what they did
