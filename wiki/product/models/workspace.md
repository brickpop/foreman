---
type: spec
status: partial
tags: [model, workspace, config]
---

# Workspace

The root folder Foreman runs on (e.g. `~/agents-ws`). It holds Foreman's own data in `.foreman/` and one subfolder per project. Foreman never clones or provisions repositories: the CEO puts them in project folders by hand.

## Config vs. state vs. content

| Kind | Where | Who edits | UI behaviour |
|---|---|---|---|
| **Project config** | `<project>/.foreman/project.yaml` (committed with the project) | CEO in their editor, or the Chief of Staff on a branch | **Read-only** in the UI. "Change with Chief of Staff" opens a guided edit that comes back as a review request. See [Project config](./project-config.md). |
| **Wiki content** | `<project>/wiki/**/*.md`, `wiki.toml` | Agents (mainly the scrum master), the CEO in their editor | Read-only prose. Exceptions, as in wikiview: toggling a checkbox, and moving a board card (rewrites one frontmatter field). See [Wiki engine](../architecture/wiki-engine.md). |
| **Team definitions** | `~/agents-ws/.foreman/teams/<team>.yaml` | The UI (Foreman owns these files), the Chief of Staff, or the editor | Editable in the UI. Changes write to the file and apply to every running instance on its next turn. See [Team definition](./team-definition.md). |
| **Workspace settings** | `~/agents-ws/.foreman/settings.yaml` (local, not shared) | The UI | Editable in the UI. See [Workspace settings](./workspace-settings.md). |
| **Secrets** | OS keychain, or `~/agents-ws/.foreman/secrets` with 0600 permissions | The UI | Write-only fields. Never written into project folders. See [Secrets](./secrets.md). |
| **Runtime state** | `~/agents-ws/.foreman/state.db` (SQLite) | Foreman | Deployments, instances, requests, team messages, work items, pull requests, read markers, UI prefs. See [Persistence](../architecture/persistence.md). |
| **Session logs** | `~/agents-ws/.foreman/logs/<agent>@<project>/<session-id>.log` | Foreman | Raw terminal byte stream: each session's history and audit trail. |

The file watcher reloads config and wiki changes within about 1 second and pushes them to clients ([API](../architecture/api.md)).

## Starting Foreman

```
foreman [<workspace-dir>] [--yes] [--no-open] [--port 8888] [--new-token]
```

- The workspace is the folder given as argument, or the current folder.
- If it has no `.foreman/` yet, Foreman asks once: `~/agents-ws isn't a Foreman workspace yet. Create .foreman/ here? [y/N]`. Anything but `y` exits without touching the disk. `--yes` answers yes (for scripts). This keeps a wrong folder (the home directory, a repo) from becoming a workspace by accident.
- On every start Foreman prints the URL with the [access token](../architecture/security.md#access-token) and opens it in the default browser; `--no-open` only prints it.
- `--port` sets the UI port (default `8888`); the interface it binds to follows the `listen` [setting](./workspace-settings.md).
- `--new-token` replaces the access token before starting, which logs every browser out.
- `--ui-dir <folder>` (development only) serves the web UI from a folder instead of the embedded build; `just dev` uses it.

## Layout

```
~/agents-ws/
├─ .foreman/
│  ├─ settings.yaml        workspace settings (listen, limits, harness commands, notifications)
│  ├─ secrets              forge tokens (0600), unless the OS keychain is used
│  ├─ teams/               team definitions (souls + playbooks)
│  │  └─ atlas.yaml
│  ├─ cos/                 Chief of Staff working directory and session
│  ├─ state.db             runtime state
│  └─ logs/                PTY logs per instance
└─ ledger/                 a project
   ├─ ledger-api/          git repo (cloned by the CEO)
   ├─ ledger-web/          git repo
   ├─ wiki/                wiki bundle (index.md, adr/, backlog/, standups/, wiki.toml)
   ├─ .sandboxes/          one clone per agent instance
   │  ├─ mara/ledger-api/
   │  └─ tove/ledger-api/
   └─ .foreman/
      └─ project.yaml      project config
```

## Discovery

- A **project** is any direct subfolder of the workspace containing `.foreman/project.yaml`.
- A subfolder without it is a **candidate**, listed by [Add a project](../features/add-project.md) with its detected git repos and whether it has a `wiki/`.
- **Repositories** are the git repos found in the project folder, unless `project.yaml` lists `repos` explicitly. `.sandboxes/` is never scanned for repos.

## Invariants

- `.sandboxes/` must be git-ignored in the project folder. Foreman adds it to `.git/info/exclude` of any enclosing repo.
- Nothing under `.foreman/` of the workspace (secrets, state, logs, team files) is ever mounted into an agent container ([Session runner](../architecture/session-runner.md)). The Chief of Staff is the only session that works on Foreman's files, and only through its own tools.
- Secrets are never written into project folders or logs.
