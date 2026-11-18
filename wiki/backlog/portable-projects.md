---
type: task
status: in-progress
priority: high
tags: [feature]
epic: install-first-run
---

# Portable projects: restore Foreman on a new machine

**Why:** the CEO, 2026-11-15: "I will install this on a new computer and I want things to be easy to sync (teams, config, project backlogs/wiki, etc.) and currently it is not." Today the CEO clones `~/.foreman/config` and each project repo by hand, and a new machine still doesn't know which projects exist: their records (name, code repos, default branches) are only in `state.db`. **This reverses** [Projects anywhere](./epics/projects-anywhere.md)'s "projects belong to the machine" (2026-10-20): projects travel; only their paths stay on the machine.

Blocked by [Remove the forge](./archive/remove-forge.md), [One Foreman folder](./foreman-folder.md) and [The wiki at the HQ's root](./archive/wiki-at-root.md): it builds on `~/foreman/{hq,projects,repos}` and on sync through each repo's `origin`.

## Decided (the CEO, 2026-11-15)

- **One machine at a time.** The CEO never runs two Foremans on the same projects at once; sync is handover and backup, not live replication.
- **What travels is the HQs, nothing else.** Foreman's HQ (`instructions.md`, `teams/`, `projects.toml`, `outcomes/`) and each project's HQ. What stays on the old machine: `~/.foreman/settings.toml` (this machine's setup), `state.db` (deployments, requests, messages, merge requests), the agents' conversations, logins, and agents' branches not pushed by the CEO. On a new machine the leads start from their wiki.
- **`projects.toml` in Foreman's HQ** holds every project's portable record; `state.db` keeps only what belongs to this machine (each code repo's path, the project's profile). One source each, no copy:

  ```toml
  [ledger]
  display_name = "Ledger"            # "" shows the id readable
  hidden = false
  hq = "git@github.com:me/ledger-hq.git"   # the HQ's origin; "" when it has none

  [ledger.repos.api]
  remote = "git@github.com:me/ledger-api.git"   # the code repo's origin when added; "" when it has none
  root_commit = "a1b2c3…"
  default_branch = "main"
  ```

  Foreman writes it on every change to a project (add, remove, rename, hide, a repo added, renamed, removed, its default changed), one commit each, like a team save. The CEO's hand edits are read on reload, like team files; a broken one is a problem, not a crash.
- **Ids:** first come, first kept. Adding a project whose id `projects.toml` already has (on this machine or not) is refused, as today.
- **Code repos: use what's there, clone what isn't.** Wherever a repo is picked (Bring here, Add a project, Repos → Add), the CEO either picks an existing folder anywhere in `$HOME` (checked by its root commit, as Locate does) or has Foreman **clone it from its remote into `~/foreman/repos/<path>`**, `<path>` being the remote URL's path without `.git` (`me/ledger-api`; a GitLab subgroup keeps its groups). Foreman clones only there. A folder already at that place holding the same root commit is used as is; another one is refused with the reason. "Foreman never clones code" goes from the specs (it was never the CEO's rule).
- **Sync is automatic, through each HQ's `origin`, with the CEO's own git credentials** (SSH agent, credential helper; no token in Foreman):
  - **On start**, for Foreman's HQ and each project's HQ with an origin: fetch, merge the remote's `main`, push. With one machine at a time this is a fast-forward; a conflict aborts the merge, leaves that HQ as it was, and shows a warning on the Dashboard naming the HQ and git's reason. It never blocks the start. Pushing on start also delivers what a crash or an offline stop left behind.
  - **On stop**: push each HQ, in parallel, a few seconds at most each; a failure is logged and the next start pushes it.
  - **Foreman never commits for anyone.** Foreman's HQ is committed on the fly (every save). A project's HQ is the lead's (`commit()`) and the CEO's (their editor): uncommitted edits there are left alone, and a stop with any says so in the log.
  - **Signing** (the CEO, 2026-11-15: "everything that can be signed, is signed. Anything Foreman changes should use my git, and obviously any repo's I approve a merge on, as well. The rest of lead's unilateral work... who cares"): every commit Foreman makes (its saves to Foreman's HQ, `projects.toml`, card moves and checkbox ticks in a project's HQ, the merges the CEO approves) uses the CEO's git, identity and signing as their config says, as today ([Technical debt → Signing](../technical-debt.md#signing-as-the-ceo-from-foremans-process)). A lead's own `commit()`s carry the agent's identity, unsigned, and are pushed as they are. Pushes and pulls use the CEO's git too.
  - An HQ with no origin, or one whose push failed, shows **Not backed up** on the Dashboard with the `git remote add origin …` line to run. Foreman has no field for it: the CEO's git sets it.
- **Restore on a new machine:** while `~/foreman/hq/` doesn't exist, the setup checklist's first step is **Foreman's HQ: Start fresh** (made as [One Foreman folder](./foreman-folder.md) says) **or Restore from a git URL** (cloned there). An `hq/` already in place (copied by hand) is used as it is.
- **Bring here:** a project in `projects.toml` with no HQ at `projects/<id>/hq/` is **not on this machine**: the Dashboard lists it apart, with **Bring here**. That clones its HQ from `hq` (or takes the folder already there, when it is a git repo), then asks for each code repo, use existing or clone (above). A repo not resolved yet is **missing**, as today, until it is. A project without an `hq` URL can't be brought: it says so.

## Decided while building (the CEO, 2026-11-16)

- **Restore replaces a pristine HQ.** Foreman still makes its HQ at first start ([One Foreman folder](./foreman-folder.md)); while it is untouched (its first commit only, no teams, no projects), the checklist offers Restore from a git URL, which replaces it with the clone. There is never a state without an HQ.
- **A project's `hq` URL is refreshed at each sync**: on start, Foreman reads each HQ's origin and updates `projects.toml` when it differs (one commit), so a `git remote add` made later is picked up.
- **No data migration in Foreman**: the CEO's records move into `projects.toml` with the one-off script ([One Foreman folder](./foreman-folder.md)); Foreman reads only the new shape. The schema still changes through numbered SQL files, squashed later (before a release).
- **Shipped in four parts**, each committed on its own: `projects.toml`; sync; cloning into `repos/`; Restore and Bring here.

## Built in part 1 (`projects.toml`), 2026-11-16

- `internal/home/records.go`: the model, `ParseRecords` (its checks), `changeRecords` (refused while the file has problems; version-guarded write; one commit `Projects: …`). The snapshot holds the records; projects are the records joined with this machine's rows (`store.Projects`: profile, repo paths; rows made on demand). Migration 027 drops the moved columns.
- A repo with no path here is `repos.<name>: not on this machine yet: Locate it`. Locating and the profile write only the database.
- `git.Origin` strips an https URL's user and password (a token there would be committed and pushed). `git -C ""` now fails (`ErrNoFolder`) instead of acting on the server's folder: a repo with no path must never run git where Foreman runs.
- A row with no entry (taken out of `projects.toml` by hand) is cleared, history included, when a new project takes that id.
- `GET /api/home` has `projectsProblems`; the Dashboard shows them.

## Built in part 2 (sync), 2026-11-16

- `git.Pull` / `git.Push` (`internal/git/sync.go`): origin and `main` only, no prompt. Pull merges only when the HQ is clean; with uncommitted changes, a fast-forward or nothing (an aborted merge can lose them); a conflict is aborted. Push never forces.
- `Home.Sync` on start, after Load and before any agent (each step bounded at 30 s), `Home.PushAll` on stop after the agents (10 s each). `Home.Backups` is the Dashboard's **Not backed up** (`GET /api/home`, `backups`).
- Taken as defaults (say if not): the sync runs before the server serves, so a slow remote delays the start up to the bounds; no "Sync now" button (a remote added by hand shows at the next start).

## Built in part 3 (cloning), 2026-11-16

- `internal/home/clone.go`: `cloneDir` (the URL's path under `repos/`; only URLs with a host), `CloneRepo` (Repos → Add, tab **Clone from a URL**), `CloneMissingRepo` (Repos → Locate, **Clone it** from the record's remote: what Bring here will use). A clone already there is reused when its origin is that URL (adding) or it holds the root commit (locating); anything else refused, untouched.
- `git.Origin` reads `remote.origin.url` as set: `get-url` applied the CEO's `insteadOf` rewrites, which would have put this machine's rewrite in `projects.toml`.
- Add a project has no repo picking of its own (a new project opens Repos), so there is nothing to add there; Bring here (part 4) uses Clone it.
- A clone runs with the home's lock held, as an HQ clone does: the UI waits on it, and other home changes wait meanwhile (bounded by git's own timeouts, none of Foreman's).

## Built in part 4 (Restore, Bring here), 2026-11-16

- `internal/home/restore.go`: `Pristine` (first commit only, clean, no team, no `projects.toml`), `Restore` (cloned into `.hq-restore`, checked: on `main`, holds `instructions.md`, `teams/` or `projects.toml`; the HQ's contents swapped, its folder kept so the watcher still follows it; the start's "no origin" warning cleared), `BringHere` (the HQ cloned from the record's `hq`; nothing left behind on failure).
- `Project.here` (its HQ on this machine) and `Project.remote` (the record's `hq`). The UI's `useProjects` is the projects here; `useAllProjects` (every record) only for names and the Dashboard's **Not on this machine**.
- The checklist's first row, **Foreman's HQ**: todo while pristine, with a URL field and **Restore**. `POST /api/home/restore`, `POST /api/projects/:id/bring`.
- Taken as defaults (say if not):
  - **No Start fresh button**: going on (a team, a project) is starting fresh; the row turns done by itself.
  - **Bring here clones the HQ only**, then opens Repos, where each repo is located or cloned one by one (**Clone it**). No "clone every repo" in one go.
  - **A project whose HQ was deleted here** reads as not on this machine too: off the sidebar, on the Dashboard with Bring here (it was a broken sidebar row).
  - **Hidden projects not here** are listed with the rest under Not on this machine.

## Edge cases (accepted)

- A merge conflict in `projects.toml` happens only if two machines changed projects without syncing between them, which one-at-a-time rules out; when it happens it is a sync warning like any other.
- A code repo's `remote` is read once, when it is added. A repo whose origin changes later keeps the old URL until the CEO edits `projects.toml` (or removes and adds the repo).
- Removing a project removes its `projects.toml` entry; its HQ's remote is untouched. Bringing it back means adding it again with Clone.
- **A clone has no submodules checked out** (`git.Clone` doesn't recurse), so a private submodule can't reach the agents from a repo Foreman cloned: [Submodules, everywhere Foreman touches a repo](./submodules-everywhere.md).
- **A restore stopped halfway** (Foreman killed between emptying the HQ and moving the clone in) leaves the HQ empty or partial: the next start makes a new HQ over an empty folder, and the CEO restores again; `.hq-restore` is cleared by the next restore. The window is a few renames.
- **The "A project" step** counts projects not on this machine: after a restore it is done though none is here yet.
- **A request or a session link to a project not here** opens "No project <id>": its history stays in `state.db` (an HQ deleted by hand), its pages need its HQ.
- Two remotes with the same path on different hosts (`github.com/me/app`, `gitlab.com/me/app`) map to one folder: the second is used if it is the same repo, refused if not ([Clones: two remotes with the same path](./clones-by-host.md)).

## Steps

- [x] `projects.toml`: its model and checks, read at start and on reload; every project change writes it (one commit); `state.db`'s project tables keep only path and profile (a migration; the CEO's records go into `projects.toml` through the move script of [One Foreman folder](./foreman-folder.md))
- [x] sync: on start (fetch, merge, push; a conflict aborted and reported), on stop (push, bounded); the Dashboard's warnings (conflict, not backed up, push failed)
- [x] clone into `~/foreman/repos/<path>`: the path from the URL, reuse by root commit, refusals; offered beside picking a folder in Bring here, Add a project and Repos → Add
- [x] restore: the checklist's first step (Start fresh, Restore from a git URL)
- [x] Bring here: projects not on this machine on the Dashboard, the HQ cloned, each repo resolved
- [x] specs: home, project-config (the record split between `projects.toml` and `state.db`), persistence, add-project, repos, project-settings, first-run, dashboard, glossary ("never clones" out), security (clone URLs, the CEO's credentials used from Foreman's process)
- [ ] tests (all but the last are in: `restore_test.go` covers restore from a URL and Bring here with a repo cloned): `projects.toml` round-trips and its problems; a project change makes one commit; an id clash refused; sync on start fast-forwards, aborts a conflict cleanly, pushes what a crash left; stop pushes with a timeout; an HQ with no origin warns; clone path from SSH, HTTPS and subgroup URLs; reuse and refusal at an occupied path; restore from a URL; Bring here with one repo picked and one cloned; a restored machine deploys a team end to end (fake harness)
