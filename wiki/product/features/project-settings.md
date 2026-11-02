---
type: spec
status: partial
tags: [feature, ui, config, teams]
---

# Project settings

The project's **Settings** section ([Project page](./project-page.md)): its configuration, read-only but for the profile its agents run on. The team on the project, which this page listed until 2026-10-31, is in [Deployment](./project-deployment.md) (the CEO: remove redundant info). Its code repos, which this page listed until 2026-11-01, are in [Repos](./repos.md#the-list) (the CEO).

## Configuration

Padding `24px 24px 48px`. One column, at most 760px wide, of four [cards](../design/design-system.md#surfaces), 24px apart (the CEO, 2026-11-02: grouped, it was "pure non-structure"). Each card is titled in its header band (600 14px), its blocks one under another with a `--line` between them, each block's title a small label (13px 500 `--tx4`):

1. **General**: Workspace, then Profile.
2. **Rules**, `project.toml` after the title (Mono 12px `--tx4`): What comes to you, Merges, then the file itself ("The file").
3. **How it works**, **collapsed** at first (a `›` in its header turns down when open; the CEO: reference text, out of the way): Where things live, then How work flows.
4. **Danger zone**, its border and title in `--err`: Archive, then Remove.

The blocks:

- **"Workspace"** (first; the CEO, 2026-10-28): the workspace's path (Mono 13px `--tx2`, selectable) and **`Copy`**, then "Its project repo, and in agents/ each agent's worktrees: look at and run their code there. Editing while an agent works turns into its uncommitted changes." ([Project config → The workspace](../models/project-config.md#the-workspace)).
  For a project made before workspaces, **"Move into a workspace"** instead (legacy): "This project lives in Foreman's home. Moved into a folder of yours, its project repo and its agents' worktrees are where you can find them and run their code. Uncommitted work and conversations come along. Stop the teams on it first.", a path field pre-filled `<projects folder>/<project>`, and **`Move`** (`Moving…` while it runs; `POST /api/projects/:id/migrate`). Done, the toast "<project> moved" ("Into <workspace>."); a refusal is a toast "Project not moved" with the reason.
- **"Where things live"**: the project's files, explained (Mono 13px / 1.7 `<pre>`), with the real paths and this project's name:

  ```
  ~/workspace/foreman/ledger/
  ├─ project/           project repo (git)
  │  ├─ project.toml    checks, gates, forge
  │  └─ wiki/           knowledge base + wiki.toml (boards)
  └─ agents/            one sandbox per agent: a worktree of each repo
  ~/.foreman/
  ├─ config/            global repo (git): teams, settings
  ├─ harness/ledger/  each agent's harness config and conversations
  └─ state.db           deployments, sessions, requests (runtime)
  ```

  A project made before workspaces shows the home's tree as it was:

  ```
  ~/.foreman/
  ├─ config/              global repo (git): teams, settings
  ├─ projects/
  │  └─ <project>/
  │     ├─ project.toml    checks, gates, forge
  │     └─ wiki/           knowledge base + wiki.toml (boards)
  ├─ sandboxes/
  │  └─ <project>/      one git worktree per agent, per repo
  └─ state.db             deployments, sessions, requests (runtime)
  ```

  The code repos are not in it: they are wherever the CEO keeps them, listed in [Repos](./repos.md#the-list) ([Home → Layout](../models/home.md#layout)).

- **"How work flows"**: four items, title (600) and body (`--tx2`, line-height 1.55):
  - **Team definition**: "Lives once on this machine, in the global repo: souls, job descriptions and the playbook. You can deploy it to as many projects as you like."
  - **Deployment**: "A team on this project, one at most. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix."
  - **Session prompt**: "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt."
  - **Enforced by Foreman**: "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (Deployment → In flight), wakes requesters when replies land, sends escalations to the Dashboard, and only lets the lead request merges."

- **"Profile"** (first in the right column; the CEO, 2026-10-18): a select of the CEO's [profiles](../architecture/session-runner.md#credentials) (`<name> · <Harness>`, Mono), the project's own selected (`default` unless set; one that no longer exists stays listed and selected), then its state, `connected` (`--ok`), `not connected` or `doesn't exist` (`--err`). Under it: "Its agents run on this harness and login, from their next start. Profiles are yours on this machine, in Settings → Agents" (a link to [Settings → Profiles](./settings.md#profiles)). Choosing saves it on the project's [record](../models/project-config.md#the-project-record) (`PUT /api/projects/:id/profile {profile}`), never in `project.toml`, which the lead can write; choosing `default` empties it. While any of the project's running agents (starting, running, idle, awaiting) still runs on another profile, a bordered line under the select says so (the CEO, 2026-10-18): "<n> agents still run on <their profiles>: they switch at the next start.", plus, when that means another harness, "On <Harness>, each starts a new conversation." Beside it, **`Restart them`** (`Restart it` for one) restarts them now (`POST /api/instances/:id/restart` each). A run from before the instance kept its profile is judged by its harness. Agents asleep or stopped aren't listed: they start on the new profile anyway. One profile for all its agents for now ([Profiles per agent](../../backlog/profiles-per-agent.md), later).
- **"What comes to you"**: grid `84px 1fr`, the kind label (colored, Mono uppercase, no border) and a line:
  - Approval: "An agent hits one of this project’s gates (always_ask_ceo below) and pauses until you answer."
  - Question: "An agent needs information only you have. It keeps working on other things meanwhile."
  - Review: "The lead calls request_merge() with the card that explains the work. Foreman runs the checks, then you review it: merge it, request changes, or dismiss it."
  - Finding: "A challenge team rates something high severity. You decide who fixes it, and when."
  - Blocked: "Raised by Foreman, not an agent: a session crashed, hit a rate limit, or has been stuck for 20 minutes."
- **"Merges"**: the forge label (`Forgejo`, `GitHub`, `Gitea` or `No forge`, 500) and a note:
  - forge `none`: "No forge. You merge requested branches into their target from their review, in the Inbox; Foreman does the merge."
  - otherwise: "Merge from a branch's review, in the Inbox, or create a PR on `<forge url>` and merge it there; Foreman notices. Nothing is pushed until you choose. The token lives in Settings, never in this file."
  - **Project repo**: when `forge.project_repo` is set, a row `project repo → <url>` with a small outline **`Sync`** button that fetches the forge's `main`, merges it in and pushes the project repo's `main` there, stopping with a message on a conflict (nothing is pushed automatically; [Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)), and the time of the last sync. **Phasing:** Sync comes with the [Git epic](../../backlog/epics/git.md); until then the button shows, disabled.
- **"The file"** (`project.toml`): a row with `<workspace>/project/project.toml · read-only` (the real path, Mono 12.5px `--tx3`). Below: "This file is config, committed in the project repo. Edit it in your editor." (`Change with Chief of Staff` comes later: [Project config with the Chief of Staff](../../backlog/cos-project-config.md).) Then the file's actual content in a selectable `<pre>`, exactly as on disk ([Project config](../models/project-config.md)).
- **"Archive this project"**: "It leaves the sidebar's projects and the Dashboard, and takes no deployment until you unarchive it. Everything is kept.", followed, while teams are deployed on it, by "<Atlas, Orion> stop first.". **`Archive project`** acts at once, or, with teams deployed, asks first: "Stop <teams> and archive <project>?", "<teams> stop on <project>: their sessions end; worktrees and branches are kept. Then <project> is archived.", **`Stop and archive`**. Archived, the block reads **"Archived"**, "It is out of the sidebar's projects and the Dashboard, and takes no deployment, until you unarchive it.", with **`Unarchive project`**. A refusal is a toast "Project not archived" ([Project config → archived](../models/project-config.md#the-project-record)).
- **"Remove this project"** (last): on an archived project, "Its record and its history go. Its workspace stays. Your code repos keep their branches."; on any other, "Archive it first: archiving keeps everything, removing deletes its history." with the button disabled. **`Remove project`** asks first: "Remove <project>?", "Its record and its history (deployments, messages, requests, merge requests, session logs, its agents' conversations) go for good. Its workspace, your code repos and their branches stay.", with two checkboxes, off at each opening: "Also delete its agents' worktrees, their uncommitted work included. Their branches stay in your code repos." and "Also delete its project repo folder, <path>: its wiki and project.toml. Unless you pushed it, it has no other copy. Kept, it's taken over by a new project in the same workspace." Removing opens the Dashboard with the toast "<project> removed"; a refusal is a toast "Project not removed" ([Add a project → Removing a project](./add-project.md#removing-a-project)).


Everything in this block is read-only but the profile, the code repos and Remove. Changes to `project.toml` on disk show within about 1 s. An invalid file shows an error banner naming the field ([Project config](../models/project-config.md#invalid-config)).
