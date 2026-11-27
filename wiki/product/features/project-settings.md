---
type: spec
status: partial
tags: [feature, ui, config, teams]
---

# Project settings

The project's **Settings** section ([Project page](./project-page.md)): its configuration, read-only but for the profile its agents run on. The team on the project, which this page listed until 2026-10-31, is in [Team](./project-team.md) (the CEO: remove redundant info). Its code repos, which this page listed until 2026-11-01, are in [Repos](./repos.md#the-list) (the CEO).

## Configuration

Padding `24px 24px 48px`. One column, at most 760px wide, of four [cards](../design/design-system.md#surfaces), 24px apart (the CEO, 2026-11-02: grouped, it was "pure non-structure"). Each card is titled in its header band (600 14px), its blocks one under another with a `--line` between them, each block's title a small label (13px 500 `--tx4`):

1. **General**: Name, Folders, Profile, then Visibility.
2. **Rules**, `project.toml` after the title (12.5px `--tx4`, as Settings says where each card is kept): What comes to you, Merges, then the file itself ("The file").
3. **How it works**, **collapsed** at first (a `›` in its header turns down when open; the CEO: reference text, out of the way): Where things live, then How work flows.
4. **Danger zone**, its border and title in `--err`: Remove.

The blocks:

- **"Name"** (the CEO, 2026-11-04): the project's [display name](../models/project-config.md#the-project-record) in a field (13.5px, placeholder its readable id, at most 80 characters); once changed, **`Cancel`** and **`Save`** (Enter too); a name another project shows already says "Another project shows as <name> already: pick another name." (`--err`) with Save off, as the server refuses it. "How it shows everywhere; empty, it shows as <Readable id>. Its id, <id>, stays: its folder, its agents' branches and its links are built from it." A refusal is a toast "Name not saved" with the reason.
- **"Folders"** (the CEO, 2026-10-28; its HQ and worktrees since 2026-11-15): two rows, **HQ** (`projects/<id>/hq`) and **Worktrees** (`worktrees/<id>`), each its label (13px `--tx4`), its path (Mono 13px `--tx2`, selectable) and **`Copy`**, then "Its HQ holds its wiki and project.toml. In its worktrees, one folder per agent: look at and run their code there. Editing while an agent works turns into its uncommitted changes." ([Project config → The project's folder](../models/project-config.md#the-projects-folder)).
- **"Where things live"**: the project's files, explained (Mono 13px / 1.7 `<pre>`), with the real paths and this project's name:

  ```
  ~/foreman/
  ├─ hq/                    Foreman's HQ (git): teams, your instructions, projects.toml
  ├─ projects/ledger/
  │  ├─ hq/                 this project's HQ (git)
  │  │  ├─ project.toml     checks, gates
  │  │  └─ wiki.toml, …     its knowledge base, at the HQ's root (boards)
  │  └─ files/              inbox/ from you, shared/ by its agents
  └─ worktrees/ledger/  one sandbox per agent: a worktree of each repo
  ~/.foreman/
  ├─ settings.toml          this machine's settings
  ├─ harness/ledger/  each agent's harness config and conversations
  └─ state.db               profile, repos' paths; deployments, sessions, requests (runtime)
  ```

  Foreman's folder is the real one (`foreman_folder`, [Settings → Folders](./settings.md#folders)), Foreman's home too. The code repos are not in it: they are wherever the CEO keeps them, listed in [Repos](./repos.md#the-list) ([Home → Layout](../models/home.md#layout)).

- **"How work flows"**: four items, title (600) and body (`--tx2`, line-height 1.55):
  - **Team definition**: "Lives once on this machine, in Foreman's HQ: souls, job descriptions and the playbook. You can deploy it to as many projects as you like."
  - **Deployment**: "A team on this project, one at most. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix."
  - **Session prompt**: "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt."
  - **Enforced by Foreman**: "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (Team → In flight), wakes requesters when replies land, sends escalations to the Dashboard, and only lets the lead request merges."

- **"Profile"** (first in the right column; the CEO, 2026-10-18): a select of the CEO's [profiles](../architecture/session-runner.md#credentials) (`<name> · <Harness>`, Mono), the project's own selected (`default` unless set; one that no longer exists stays listed and selected), then its state, `connected` (`--ok`), `not connected` or `doesn't exist` (`--err`). Under it: "Its agents run on this harness and login, from their next start. Profiles are yours on this machine, in Settings → Agents" (a link to [Settings → Profiles](./settings.md#profiles)). Choosing saves it on the project's [record](../models/project-config.md#the-project-record) (`PUT /api/projects/:id/profile {profile}`), never in `project.toml`, which the lead can write; choosing `default` empties it. While any of the project's running agents (starting, running, idle, awaiting) still runs on another profile, a bordered line under the select says so (the CEO, 2026-10-18): "<n> agents still run on <their profiles>: they switch at the next start.", plus, when that means another harness, "On <Harness>, each starts a new conversation." Beside it, **`Restart them`** (`Restart it` for one) restarts them now (`POST /api/instances/:id/restart` each). A run from before the instance kept its profile is judged by its harness. Agents asleep or stopped aren't listed: they start on the new profile anyway. One profile for all its agents for now ([Profiles per agent](../../backlog/profiles-per-agent.md), later).
- **"Visibility"** (the CEO, 2026-11-13): "Hide it to keep the project list and the Dashboard to what you work on: it moves under More. Everything else works as usual, a team on it included." and **`Hide project`**; hidden, "Hidden: it is under More in the project list, and off the Dashboard. Everything else works as usual." and **`Show project`** (`PUT /api/projects/:id/hidden`, [Project config → hidden](../models/project-config.md#the-project-record)). A refusal is a toast "Project not hidden" (or "not shown").
- **"What comes to you"**: grid `84px 1fr`, the kind label (colored, Mono uppercase, no border) and a line:
  - Approval: "An agent hits one of this project’s gates (always_ask_ceo below) and pauses until you answer."
  - Question: "An agent needs information only you have. It keeps working on other things meanwhile."
  - Review: "The lead calls request_merge() with the card that explains the work. Foreman runs the checks, then you review it: merge it, request changes, or dismiss it."
  - Finding: "A challenge team rates something high severity. You decide who fixes it, and when."
  - Blocked: "Raised by Foreman, not an agent: a session crashed, hit a rate limit, or has been stuck for 20 minutes."
- **"Merges"**: "You merge requested branches into their target from their review, in the Inbox; Foreman does the merge. Nothing is pushed: you push with your own git."
- **"The file"** (`project.toml`): a row with `<hq>/project.toml · read-only` (the real path, Mono 12.5px `--tx3`). Below: "This file is config, committed in the HQ. Edit it in your editor." (`Change with Chief of Staff` comes later: [Project config with the Chief of Staff](../../backlog/cos-project-config.md).) Then the file's actual content in a selectable `<pre>`, exactly as on disk ([Project config](../models/project-config.md)).
- **"Remove this project"** (last): "Its record and its history go. Its HQ and files stay. Your code repos keep their branches."; while a team is deployed on it, "Stop <teams> on it first (its Team section)." with the button disabled. **`Remove project`** asks first: "Remove <project>?", "Its record and its history (deployments, messages, requests, merge requests, session logs, its agents' conversations) go for good. Its files, your code repos and their branches stay.", with two checkboxes, off at each opening: "Also delete its agents' worktrees, their uncommitted work included. Their branches stay in your code repos." and "Also delete its HQ, <path>: its wiki and project.toml. Unless you pushed it, it has no other copy. Kept, a new project of the same name takes it over." Removing opens the Dashboard with the toast "<project> removed"; a refusal is a toast "Project not removed" ([Add a project → Removing a project](./add-project.md#removing-a-project)).


Everything in this block is read-only but the profile, the code repos and Remove. Changes to `project.toml` on disk show within about 1 s. An invalid file shows an error banner naming the field ([Project config](../models/project-config.md#invalid-config)).
