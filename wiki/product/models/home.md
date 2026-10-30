---
type: spec
status: partial
tags: [model, home, config]
---

# Home

Foreman's folder on this machine, `~/.foreman` (or `--home`), on macOS and Linux alike. One Foreman runs per machine: `server.lock` keeps a second one from starting, and that one prints the first one's URL instead (the CEO, 2026-10-20). The home holds the **global repo** (`config/`), each agent's private **harness folder** (`harness/<project>/<agent>/`), and Foreman's state. Nothing else under the home is in a repo.

A project's own files are not in the home: they are in its **workspace**, a folder of the CEO's (`~/workspace/foreman/<name>/` by default) holding its project repo and its agents' sandboxes, where the CEO can find them and run the agents' code (the CEO, 2026-10-28; [Project config → The workspace](./project-config.md#the-workspace)). A project is a record plus its workspace, and its code repos live anywhere in the CEO's home folder. A project made before workspaces still lives in the home (`projects/<name>/`, `sandboxes/<name>/`) until the CEO moves it (legacy: [Remove the workspace migration](../../backlog/remove-workspace-migration.md)). Foreman never clones code: the repos it creates are the global repo, each project repo (new, or cloned from a URL or a local path), and, on the CEO's request, a new **empty code repo** at a path the CEO picks (a README in a first commit, on `main`).

## Config vs. state vs. content

| Kind | Where | Who edits | UI behaviour |
|---|---|---|---|
| **Project record** | `state.db`: its name, its code repos (name, path, root commit), its profile | The UI ([Project settings](../features/project-settings.md)) | See [Project config → The project record](./project-config.md#the-project-record). |
| **Project config** | `<workspace>/project/project.toml`, in the project repo | CEO in their editor | **Read-only** in the UI. Changes by the Chief of Staff come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). See [Project config](./project-config.md). |
| **Project knowledge base** | `<workspace>/project/wiki/` (`wiki.toml`, board, product, ADRs), in the project repo | The lead commits through `commit()` (no review); other agents through `contribute`; the CEO in their editor | Read-only prose. Exceptions, as in wikiview: toggling a checkbox, and moving a board card (rewrites one frontmatter field). See [Wiki engine](../architecture/wiki-engine.md). |
| **Team definitions** | `config/teams/<team>.toml`, in the global repo | The UI (one commit per save, the Chief of Staff's proposals included), or the editor | Editable in the UI. Changes write to the file and apply to every running instance on its next turn. See [Team definition](./team-definition.md). |
| **Global settings** | `config/settings.toml`, in the global repo | The UI (one commit per save) | Editable in the UI. See [Global settings](./global-settings.md). |
| **Secrets** | OS keychain, or a 0600 file in the home, outside the global repo | The UI | Write-only fields. Never written into a repo. See [Secrets](./secrets.md). |
| **Runtime state** | `state.db` (SQLite) | Foreman | Projects' records, deployments, instances, requests, team messages, work items, merge requests, read markers. See [Persistence](../architecture/persistence.md). |
| **Session logs** | `logs/<agent>@<project>/<session-id>.log` | Foreman | Raw terminal byte stream: each session's history and audit trail. |

The file watcher reloads `config/` and each project's `project.toml` within about 1 second and pushes the change to clients ([API](../architecture/api.md)). Code repos are never watched: they are checked on load, on every reload and before each start.

### The global repo

`config/` is a git repo, made (`git init`, a first commit "Global repo") the first time Foreman starts on the home; teams and settings put there by hand come in with that commit. It versions what the CEO owns: `teams/` and `settings.toml`. Its `.gitignore` holds `*.tmp` (the files a save writes before its rename); when a newer Foreman ignores a new line, it is added at start, in one commit, keeping the CEO's lines. Every save from the UI is **one commit**, and so is every Chief of Staff proposal the CEO confirms: the Chief of Staff never writes here itself ([Chief of Staff](../features/chief-of-staff.md#how-it-runs)). Nothing is pushed automatically: with a forge set, the CEO pushes `main` there with **Sync** in [Settings](../features/settings.md#forge-accounts) ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)).

The global repo is its own subfolder, not the home itself, so that git run in a legacy sandbox (under `sandboxes/`) never finds it as an enclosing repo (the CEO, 2026-10-20).

All of Foreman's config files are **TOML**. A save rewrites the whole file, so hand-written comments don't survive it; git keeps every earlier version.

## Starting Foreman

```
foreman [--home <dir>] [--no-open] [--port 8888] [--new-token]
```

- Foreman takes no folder: given one, it refuses to start and says projects are added in its UI.
- The folder it is started in is where the folder browser opens, a convenience: run Foreman from `~/dev/aragon` to start picking repos there. Workspaces and code repos may be anywhere in the CEO's home folder, `$HOME`, never inside Foreman's home (the CEO, 2026-10-28, replacing the start-folder bound of 2026-10-20; [Add a project → Picking repos](../features/add-project.md#picking-repos)).
- On every start Foreman prints the URL with the [access token](../architecture/security.md#access-token) and opens it in the default browser; `--no-open` only prints it. A second `foreman` while one runs prints the first one's URL (and opens it) and exits, touching nothing.
- `--home <dir>` runs on another home (tests, `just dev`); a relative one is made absolute.
- `--port` sets the UI port, over `port` in [settings](./global-settings.md) (default `8888`). Foreman binds `127.0.0.1` only: it is never reachable from another machine (the CEO, 2026-10-30).
- `--new-token` replaces the access token before starting, which logs every browser out.
- `--ui-dir <folder>` (development only) serves the web UI from a folder instead of the embedded build; `just dev` uses it.
- `--fake-harness <program>` (development only) runs agents and harness logins on that program (`cmd/fakeharness`) instead of Claude Code, so a team deploys without a login. Direct engine only: on Podman, Foreman refuses to start with it (a container can't run a host binary). `just dev fake` uses it; `just dev` doesn't.

## Layout

```
~/.foreman/
├─ config/                 the global repo (git)
│  ├─ .gitignore           *.tmp
│  ├─ settings.toml        global settings (port, limits, engine, forges, notifications)
│  └─ teams/               team definitions (souls + playbooks)
│     └─ atlas.toml
├─ harness/
│  └─ ledger/
│     └─ mara/             the harness's own files for this agent (0700): config, conversations
├─ cos/                    Chief of Staff working directory and session
├─ profiles/               the CEO's profiles: a harness and its login each
├─ images/                 the agent images' build state
├─ credentials/            agent-link (the secret that signs agents' tokens)
├─ logs/                   PTY logs per instance
├─ access-token            the access token (0600)
├─ preferences.toml        the CEO's preferences (theme, terminal font)
├─ server.lock             one Foreman per machine; holds the running one's URL
└─ state.db                runtime state, and the projects' records
```

A project's workspace, in the projects folder ([Global settings](./global-settings.md) `projects_folder`) unless the CEO picked another:

```
~/workspace/foreman/
└─ ledger/                 the project's workspace
   ├─ project/             the project repo (git), made by New project: always here
   │  ├─ project.toml      project config
   │  ├─ AGENTS.md
   │  └─ wiki/             knowledge base (wiki.toml, index.md, backlog/, product/, adr/, inbox/)
   └─ agents/
      └─ mara/             mara's sandbox
         ├─ ledger-api/    a git worktree of each code repo, on the agent's branch
         └─ project/       the project repo's mount point (writable for the lead only)
```

The code repos are wherever the CEO keeps them, e.g. `~/dev/ledger-api`. A project made before workspaces has its project repo at `~/.foreman/projects/<name>/` and its sandboxes at `~/.foreman/sandboxes/<name>/<agent>/`, each with its harness folder inside (`.harness/`), until it is moved ([Project settings](../features/project-settings.md)).

## Invariants

- The home is 0700: it holds the access token, the logins and the secrets.
- Nothing in the home but an agent's own harness folder is mounted into its container, beside its sandbox and its project's repo in the workspace, plus each code repo's `.git` read-only at its own path (so the worktree's link resolves) and the login of the project's profile ([Session runner → Credentials](../architecture/session-runner.md#credentials)). The one exception is the Chief of Staff: its own `cos/` and `config/teams/` read-only; it changes team files only through `propose_team`, which Foreman writes on the CEO's click ([Chief of Staff → How it runs](../features/chief-of-staff.md#how-it-runs)).
- No code repo or workspace is inside the home, and the home is never added as a repo. Both are inside `$HOME`.
- A workspace isn't inside a code repo (its worktrees would show in it), nor inside or around another project's.
- Secrets are never written into a repo or a log.
