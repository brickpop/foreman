---
type: spec
status: partial
tags: [model, workspace, config]
---

# Workspace

The root folder Foreman runs on (e.g. `~/agents-ws`). It holds Foreman's own data in `.foreman/` and one subfolder per project. Foreman never clones code repositories: the CEO puts them in project folders by hand. The repos Foreman creates are the **workspace repo** (`.foreman/`), each project's **project repo** (`<project>/project/`), and, on the CEO's request, a new **empty code repo** (a README in a first commit, on the project's `main`) for greenfield work ([Add a project](../features/add-project.md), [Project settings](../features/project-settings.md)).

## Config vs. state vs. content

| Kind | Where | Who edits | UI behaviour |
|---|---|---|---|
| **Project config** | `<project>/project/project.toml`, in the project repo | CEO in their editor | **Read-only** in the UI. Changes by the Chief of Staff come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). See [Project config](./project-config.md). |
| **Project knowledge base** | `<project>/project/wiki/` (`wiki.toml`, board, product, ADRs), in the project repo | The lead commits through `commit()` (no review); other agents through `contribute`; the CEO in their editor | Read-only prose. Exceptions, as in wikiview: toggling a checkbox, and moving a board card (rewrites one frontmatter field). See [Wiki engine](../architecture/wiki-engine.md). |
| **Team definitions** | `~/agents-ws/.foreman/teams/<team>.toml`, in the workspace repo | The UI (one commit per save, the Chief of Staff's proposals included), or the editor | Editable in the UI. Changes write to the file and apply to every running instance on its next turn. See [Team definition](./team-definition.md). |
| **Workspace settings** | `~/agents-ws/.foreman/settings.toml`, in the workspace repo | The UI (one commit per save) | Editable in the UI. See [Workspace settings](./workspace-settings.md). |
| **Secrets** | OS keychain, or `~/agents-ws/.foreman/secrets` with 0600 permissions (git-ignored) | The UI | Write-only fields. Never written into project folders. See [Secrets](./secrets.md). |
| **Runtime state** | `~/agents-ws/.foreman/state.db` (SQLite, git-ignored) | Foreman | Deployments, instances, requests, team messages, work items, merge requests, read markers, UI prefs. See [Persistence](../architecture/persistence.md). |
| **Session logs** | `~/agents-ws/.foreman/logs/<agent>@<project>/<session-id>.log` (git-ignored) | Foreman | Raw terminal byte stream: each session's history and audit trail. |

The file watcher reloads config and wiki changes within about 1 second and pushes them to clients ([API](../architecture/api.md)).

### The workspace repo

`.foreman/` is a git repo, created (`git init`) with the workspace. It versions the files the CEO owns there (`teams/`, `settings.toml`); its `.gitignore` keeps out `state.db*` (the database and its WAL side files), `logs/`, `secrets`, `credentials/`, `access-token` and `cos/` (the Chief of Staff's own folder); a workspace repo from an older Foreman gets the lines it lacks added, in one commit, at start. Every save from the UI is **one commit**, and so is every Chief of Staff proposal the CEO confirms: the Chief of Staff never writes here itself ([Chief of Staff](../features/chief-of-staff.md#how-it-runs)). Nothing is pushed automatically: with a forge set, the CEO pushes `main` there with **Sync** in [Settings](../features/settings.md#forge-accounts) ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)).

All of Foreman's config files are **TOML**. A save rewrites the whole file, so hand-written comments don't survive it; git keeps every earlier version.

## Starting Foreman

```
foreman [<workspace-dir>] [--yes] [--no-open] [--port 8888] [--new-token]
```

- The workspace is the folder given as argument, or the current folder.
- If it has no `.foreman/` yet, Foreman asks once: `~/agents-ws isn't a Foreman workspace yet. Create .foreman/ here? [y/N]`. Anything but `y` exits without touching the disk. `--yes` answers yes (for scripts). This keeps a wrong folder (the home directory, a repo) from becoming a workspace by accident. Creating it makes `.foreman/` the [workspace repo](#the-workspace-repo).
- On every start Foreman prints the URL with the [access token](../architecture/security.md#access-token) and opens it in the default browser; `--no-open` only prints it.
- `--port` sets the UI port (default `8888`); the interface it binds to follows the `listen` [setting](./workspace-settings.md).
- `--new-token` replaces the access token before starting, which logs every browser out.
- `--ui-dir <folder>` (development only) serves the web UI from a folder instead of the embedded build; `just dev` uses it.
- `--fake-harness <program>` (development only) runs agents and harness logins on that program (`cmd/fakeharness`) instead of Claude Code, so a team deploys without a login. Direct engine only: on Podman, Foreman refuses to start with it (a container can't run a host binary). `just dev fake` uses it; `just dev` doesn't.

## Layout

```
~/agents-ws/
├─ .foreman/               the workspace repo (git)
│  ├─ .gitignore           state.db*, logs/, secrets, credentials/, access-token
│  ├─ settings.toml        workspace settings (listen, limits, engine, notifications)
│  ├─ teams/               team definitions (souls + playbooks)
│  │  └─ atlas.toml
│  ├─ cos/                 Chief of Staff working directory and session
│  ├─ secrets              forge tokens (0600), unless the OS keychain is used
│  ├─ credentials/         agent-link (the secret that signs agents' tokens); and, from before profiles, a harness login until Foreman makes it a profile
│  ├─ access-token         the per-install access token (0600)
│  ├─ state.db             runtime state
│  └─ logs/                PTY logs per instance
└─ ledger/                 a project
   ├─ project/             the project repo (git), created by Add a project
   │  ├─ project.toml      project config
   │  └─ wiki/             knowledge base (wiki.toml, index.md, backlog/, product/, adr/, inbox/)
   ├─ ledger-api/          code repo (cloned by the CEO)
   ├─ ledger-web/          code repo
   └─ .sandboxes/          one sandbox per agent: a git worktree of each code repo
      ├─ mara/ledger-api/
      └─ tove/ledger-api/
```

## Discovery

- A **project** is any direct subfolder of the workspace containing `project/project.toml`.
- A subfolder without it is a **candidate**, listed by [Add a project](../features/add-project.md) with its detected git repos.
- **Code repositories** are the git repos found in the project folder other than `project/`, unless `project.toml` lists `repos` explicitly. `.sandboxes/` is never scanned for repos.

## Invariants

- `.sandboxes/` must be git-ignored in the project folder. Foreman adds it to `.git/info/exclude` of any enclosing repo.
- Nothing under `.foreman/` of the workspace (secrets, state, logs, team files) is ever mounted into an agent container. The harnesses' logins aren't in the workspace at all: they are the CEO's profiles, shared by every workspace, and the login file of the project's profile is mounted ([Session runner → Credentials](../architecture/session-runner.md#credentials)). The one exception is the Chief of Staff: its own `.foreman/cos/` and `.foreman/teams/` read-only; it changes team files only through `propose_team`, which Foreman writes on the CEO's click ([Chief of Staff → How it runs](../features/chief-of-staff.md#how-it-runs)).
- Secrets are never written into project folders or logs.
