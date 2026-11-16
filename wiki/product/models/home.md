---
type: spec
status: partial
tags: [model, home, config]
---

# Home

Foreman on this machine, on macOS and Linux alike, is two folders (the CEO, 2026-11-08): its **home**, `~/.foreman` (or `--home`), holding Foreman's machinery, and **[Foreman's folder](#foremans-folder)**, `~/foreman` by default, holding what the CEO owns. One Foreman runs per machine: `server.lock` keeps a second one from starting, and that one prints the first one's URL instead (the CEO, 2026-10-20).

The home holds this machine's [settings](./global-settings.md) (`settings.toml`, a plain file, in no repo), each agent's private **harness folder** (`harness/<project>/<agent>/`), and Foreman's state. Nothing under the home is in a repo. Foreman's folder holds **[Foreman's HQ](#foremans-hq)** (the teams, the CEO's instructions and the projects' records), each project's **HQ** and files ([Project config → The project's folder](./project-config.md#the-projects-folder)), and the agents' worktrees. A project is a record plus its HQ and files, and its code repos live anywhere in the CEO's home folder. Foreman never clones code: the repos it creates are Foreman's HQ, each project's HQ (new, or cloned from a URL or a local path), and, on the CEO's request, a new **empty code repo** at a path the CEO picks (a README in a first commit, on `main`).

## Config vs. state vs. content

| Kind | Where | Who edits | UI behaviour |
|---|---|---|---|
| **Project record** | `hq/projects.toml`, in Foreman's HQ: its id, display name, hidden, its HQ's origin, its code repos (name, origin, root commit, default branch). `state.db`: this machine's part, its profile and each repo's path | The UI ([Project settings](../features/project-settings.md), the code repos from [Repos](../features/repos.md#the-list)), one commit per change; or the editor | See [Project config → The project record](./project-config.md#the-project-record). |
| **Project config** | `projects/<project>/hq/project.toml`, in the project's HQ | CEO in their editor | **Read-only** in the UI. Changes by the Chief of Staff come later ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). See [Project config](./project-config.md). |
| **Project knowledge base** | `projects/<project>/hq/` (`wiki.toml`, board, product, ADRs), the project's HQ, a wiki at its root | The lead commits through `commit()` (no review); other agents through `contribute`; the CEO in their editor | Read-only prose. Exceptions, as in wikiview: toggling a checkbox, and moving a board card (rewrites one frontmatter field). See [Wiki engine](../architecture/wiki-engine.md). |
| **Team definitions** | `hq/teams/<team>.toml`, in Foreman's HQ | The UI (one commit per save, the Chief of Staff's proposals included), or the editor | Editable in the UI. Changes write to the file and apply to every running instance on its next turn. See [Team definition](./team-definition.md). |
| **The CEO's instructions** | `hq/instructions.md`, in Foreman's HQ | The UI (one commit per save), or the editor | Editable in the UI ([Settings](../features/settings.md#instructions-for-every-agent)). Added to every agent's [session prompt](../architecture/session-prompt.md#the-ceos-instructions). |
| **Global settings** | `~/.foreman/settings.toml`, a plain file | The UI (no commit), or the editor | Editable in the UI. See [Global settings](./global-settings.md). |
| **Runtime state** | `state.db` (SQLite) | Foreman | This machine's part of each project, deployments, instances, requests, team messages, work items, merge requests, read markers. See [Persistence](../architecture/persistence.md). |
| **Session logs** | `logs/<agent>@<project>/<session-id>.log` | Foreman | Raw terminal byte stream: each session's history and audit trail. |

The file watcher reloads `settings.toml`, Foreman's HQ (its `teams/`, `instructions.md` and `projects.toml`), `projects/` and each project's HQ within about 1 second and pushes the change to clients ([API](../architecture/api.md)). Code repos are never watched: they are checked on load, on every reload and before each start.

### Foreman's HQ

`hq/` in Foreman's folder is a git repo, made the first time Foreman starts on the folder: `git init`, a `.gitignore` holding `*.tmp` (the files a save writes before its rename) and `.wiki/` (the wiki's index), `teams/`, an empty `instructions.md`, and a wiki at its root as a project's HQ has (`wiki.toml`, ignoring `instructions.md`, which is no entry; `index.md`), in one commit "Foreman's HQ" (what was put there by hand comes in with it, never overwritten). A start that stopped before that commit finishes it at the next start. Nothing reads its wiki yet: the [Director's board](../../backlog/director-board.md) (`outcomes/`) comes with the Director. It versions what the CEO owns across machines: the team files, the CEO's instructions and the projects' records (`projects.toml`, [Project config](./project-config.md#projectstoml)); of all the settings, only the instructions travel between machines (the CEO, 2026-11-15). Every save from the UI is **one commit**, and so is every Chief of Staff proposal the CEO confirms: the Chief of Staff never writes here itself ([Chief of Staff](../features/chief-of-staff.md#how-it-runs)). A hand edit is read on reload. Nothing is pushed automatically: the CEO pushes it with their own git (its automatic sync comes with [Portable projects](../../backlog/portable-projects.md), planned).

All of Foreman's config files are **TOML**, but `instructions.md`, which is prose. A save rewrites the whole file, so hand-written comments don't survive it; in an HQ, git keeps every earlier version (`settings.toml` keeps none).

## Starting Foreman

```
foreman [--home <dir>] [--no-open] [--port 8888] [--new-token]
```

- Foreman takes no folder: given one, it refuses to start and says projects are added in its UI.
- The folder it is started in is where the folder browser opens, a convenience: run Foreman from `~/dev/aragon` to start picking repos there. Code repos may be anywhere in the CEO's home folder, `$HOME`, never inside Foreman's home nor in Foreman's own folders (the CEO, 2026-10-28, replacing the start-folder bound of 2026-10-20; [Add a project → Picking repos](../features/add-project.md#picking-repos)).
- [Foreman's folder](#foremans-folder) is read from `settings.toml` at start; one Foreman can't use doesn't let it start, saying why.
- On every start Foreman prints the URL with the [access token](../architecture/security.md#access-token) and opens it in the default browser; `--no-open` only prints it. A second `foreman` while one runs prints the first one's URL (and opens it) and exits, touching nothing.
- `--home <dir>` runs on another home (tests, `just dev`); a relative one is made absolute.
- `--port` sets the UI port, over `port` in [settings](./global-settings.md) (default `8888`). Foreman binds `127.0.0.1` only: it is never reachable from another machine (the CEO, 2026-10-30).
- `--new-token` replaces the access token before starting, which logs every browser out.
- `--ui-dir <folder>` (development only) serves the web UI from a folder instead of the embedded build; `just dev` uses it.
- `--fake-harness <program>` (development only) runs agents and harness logins on that program (`cmd/fakeharness`) instead of Claude Code, so a team deploys without a login. Direct engine only: on Podman, Foreman refuses to start with it (a container can't run a host binary). `just dev fake` uses it; `just dev` doesn't.

## Foreman's folder

`foreman_folder` in `settings.toml` ([Global settings](./global-settings.md)), `~/foreman` by default, kept as a `~` path. Foreman derives every path in it from the folder and a project's id: there is no per-project folder to pick (the CEO, 2026-11-08). It is read **at start only**, so a change takes effect at the next start. After `~` it must be absolute, inside `$HOME` (agents reach it there; on macOS, what the Podman machine shares), and neither inside Foreman's home nor around it; otherwise Foreman doesn't start, saying why. It is made if missing.

The UI doesn't change it (the CEO, 2026-11-15): [Settings → System](../features/settings.md) shows it read-only, beside the home. To move it, the CEO stops Foreman, moves the folder by hand and edits `settings.toml`. `just dev` uses its own, `.dev/<harness>-folder`.

What it holds splits by what the CEO keeps: `hq/` and `projects/` are what to back up; `worktrees/` is what can go, since every commit is in the code repo and Foreman makes a missing worktree again when its agent opens that repo ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)). Removing a worktree loses only what an agent left uncommitted.

## Layout

```
~/foreman/                 Foreman's folder (foreman_folder)
├─ hq/                     Foreman's HQ (git, a wiki at its root)
│  ├─ .gitignore           *.tmp
│  ├─ instructions.md      the CEO's instructions to every agent
│  ├─ projects.toml        every project's record (written by Foreman, one commit each)
│  └─ teams/               team definitions (souls + playbooks)
│     └─ atlas.toml
├─ projects/
│  └─ ledger/              the project's folder
│     ├─ hq/               the project's HQ (git), made by New project
│     │  ├─ project.toml   project config
│     │  ├─ AGENTS.md
│     │  └─ wiki/          knowledge base (wiki.toml, index.md, backlog/, product/, adr/, inbox/)
│     └─ files/            shared with the agents, whatever the team
│        ├─ inbox/         the CEO's files for them (read-only for them)
│        └─ shared/        their data that outlives a task and a team
└─ worktrees/
   └─ ledger/
      └─ mara/             mara's sandbox
         └─ ledger-api/    a git worktree of each code repo it opened

~/.foreman/                Foreman's home
├─ settings.toml           this machine's settings (no git)
├─ harness/
│  └─ ledger/
│     └─ mara/             the harness's own files for this agent (0700): config, conversations
├─ cos/                    Chief of Staff working directory and session
├─ profiles/               the CEO's profiles: a harness and its login each
├─ images/                 the agent images' build state
├─ credentials/            agent-link (the secret that signs agents' tokens)
├─ logs/                   PTY logs per instance
├─ access-token            the access token (0600)
├─ server.lock             one Foreman per machine; holds the running one's URL
└─ state.db                runtime state, and this machine's part of each project
```

The code repos are wherever the CEO keeps them, e.g. `~/dev/ledger-api`. Foreman's folder itself is no repo.

## Invariants

- The home is 0700: it holds the access token, the agent-link secret and the logins.
- Nothing in the home but an agent's own harness folder is mounted into its container, beside its sandbox, its project's HQ and its project's `files/inbox` (read-only) and `files/shared`, plus each code repo's `.git` read-only at its own path (so the worktree's link resolves) and the login of the project's profile ([Session runner → Credentials](../architecture/session-runner.md#credentials)). The one exception is the Chief of Staff: its own `cos/`, and Foreman's HQ's `teams/` read-only; it changes team files only through `propose_team`, which Foreman writes on the CEO's click ([Chief of Staff → How it runs](../features/chief-of-staff.md#how-it-runs)).
- Neither a code repo nor Foreman's folder is inside the home, and the home is never added as a repo. Both are inside `$HOME`.
- No code repo is in Foreman's HQ, `projects/` or `worktrees/` (added, created or located, through a symlink too): the folder browser marks them as Foreman's.
- The access token and the agent-link secret are never written into a repo or a log.
