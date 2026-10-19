---
type: spec
status: planned
tags: [feature, ui, config, teams]
---

# Project settings

The project's **Settings** tab: the teams working on it (runtime state) and its configuration (read-only, but for the profile its agents run on). The project header and tabs are described in [Wiki reader](./wiki-reader.md#project-header-and-tabs). Two stacked blocks.

## 1. Teams on this project

Padding `24px 40px 64px`.

- Title "Teams on this project" followed by `· runtime state, not config` (400 12.5px `--tx4`). Subline: `Talk to a lead and they'll delegate. Open anyone else for a direct instruction.` Right: an outline **`Deploy a team`** button, opening [Deploy a team](./deploy-team.md) with this project preset.
- Per [deployment](../models/deployment.md): the team name (600 16px), the goal (13.5px `--tx3`), and a raised **`Open sessions`** button ([Sessions](./sessions.md) on this deployment, lead pane).
- A panel of member rows (grid `auto 150px 1fr auto`, padding `12px 16px`), lead first:
  - Avatar 32 with status dot (`dotBorder --panel`).
  - Name (600) and role (12.5px `--tx4`); the lead's role reads `<Role> · talk here`.
  - The current task (13.5px `--tx2`, ellipsis) and the **activity line**, what its harness last reported doing, then ` · ` and the time (Mono 12px `--tx4`, ellipsis): `Bash: go test ./...`, `Editing api/invoice.go`, `Finished its turn · 10:42` ([Harness adapters → Activity lines](../architecture/harness-adapters.md#activity-lines)). It follows the `instance.doing` event; a run starts without one. Not the terminal's last line (decided by the CEO, 2026-10-18): a harness's screen ends in its own footer (Claude Code: `⏵⏵ bypass permissions on`), and finding the line that matters would mean reading its layout. **Phasing:** the task note isn't set by anything yet, so a row shows the activity line only.
  - `Open →` (12.5px `--tx3`). A click anywhere on the row opens that session.
- **Empty**: `No team is deployed here yet. Deploy one and each member gets its own worktree of this project's repos.`

## 2. Configuration

Padding `28px 48px 64px`. A grid of columns (`repeat(auto-fit, minmax(340px, 1fr))`, gap 36).

**Left column** (max 520px):

- **"Workspace"**: the folder tree, explained (Mono 13px / 1.7 `<pre>`), with this project's real repo names:

  ```
  ~/agents-ws/
  ├─ .foreman/          workspace repo (git)
  │  ├─ teams/          team definitions (souls, playbooks)
  │  ├─ cos/            Chief of Staff (its own folder)
  │  └─ state.db        deployments, sessions, requests (runtime)
  └─ <project>/
     ├─ project/        project repo (git)
     │  ├─ project.toml repos, checks, gates, forge
     │  └─ wiki/        knowledge base + wiki.toml (boards)
     ├─ <repo1>/
     ├─ <repo2>/
     └─ .sandboxes/     one git worktree per agent, per repo
  ```

- **"How work flows"**: four items, title (600) and body (`--tx2`, line-height 1.55):
  - **Team definition**: "Lives once in the workspace: souls, job descriptions and the playbook. You can deploy it to as many projects as you like."
  - **Deployment**: "A team on this project with a goal. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix."
  - **Session prompt**: "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt."
  - **Enforced by Foreman**: "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (that’s In flight, in Repositories), wakes requesters when replies land, sends escalations to Home, and only lets the lead request merges to main."

**Right column**:

- **"Profile"** (first in the right column; the CEO, 2026-10-18): a select of the CEO's [profiles](../architecture/session-runner.md#credentials) (`<name> · <Harness>`, Mono), the project's own selected (`default` unless set; one that no longer exists stays listed and selected), then its state, `connected` (`--ok`), `not connected` or `doesn't exist` (`--err`). Under it: "Its agents run on this harness and login, from their next start. Profiles are yours on this machine, in Settings → Global" (a link to [Settings → Profiles](./settings.md#profiles)). Choosing saves `project_profiles` in the workspace's [settings](../models/workspace-settings.md#schema), never `project.toml`; choosing `default` removes the project's line. While any of the project's running agents (starting, running, idle, awaiting) still runs on another profile, a bordered line under the select says so (the CEO, 2026-10-18): "<n> agents still run on <their profiles>: they switch at the next start.", plus, when that means another harness, "On <Harness>, each starts a new conversation; a lead gets its goal again." Beside it, **`Restart them`** (`Restart it` for one) restarts them now (`POST /api/instances/:id/restart` each). A run from before the instance kept its profile is judged by its harness. Agents asleep or stopped aren't listed: they start on the new profile anyway. One profile for all its agents for now ([Profiles per agent](../../backlog/profiles-per-agent.md), later).
- **"Code repos"**: the repos as a folder tree under `<project>/`, one per line (`├─ <repo>/`, the last `└─ <repo>/`), Mono, like the Workspace tree (the CEO, 2026-10-17); with none, in `--err`: `None yet: agents need at least one repo to work on. Clone one into <project>/, or create an empty one here.` Then a Mono input (placeholder `new-repo`) and a small **`New repo`** button: `POST /api/projects/:id/repos {name}` creates it as [Add a project](./add-project.md) does (empty, a first commit, on the project's `main`); a refusal (a bad or taken name) shows under it. When `project.toml` lists its `repos`, a new folder wouldn't count, so the form is replaced by `project.toml lists its repos: add a new one there, after creating it in <project>/.`
- **"What comes to you"**: grid `84px 1fr`, the kind label (colored, Mono uppercase, no border) and a line:
  - Approval: "An agent hits one of this project’s gates (always_ask_ceo below) and pauses until you answer."
  - Question: "An agent needs information only you have. It keeps working on other things meanwhile."
  - Review: "The lead calls request_merge() with the card that explains the work. Foreman runs the checks, then you choose: Merge to main, Create PR on your forge, or Reject."
  - Finding: "A challenge team rates something high severity. You decide who fixes it, and when."
  - Blocked: "Raised by Foreman, not an agent: a session crashed, hit a rate limit, or has been stuck for 20 minutes."
- **"Merges"**: the forge label (`Forgejo`, `GitHub`, `Gitea` or `No forge`, 500) and a note:
  - forge `none`: "No forge. You merge requested branches into main from Home or the project's Overview; Foreman does the merge."
  - otherwise: "Merge to main from Home or the project's Overview, or create a PR on `<forge url>` and merge it there; Foreman notices. Nothing is pushed until you choose. The token lives in your workspace Settings, never in this file."
  - **Project repo**: when `forge.project_repo` is set, a row `project repo → <url>` with a small outline **`Sync`** button that fetches the forge's `main`, merges it in and pushes the project repo's `main` there, stopping with a message on a conflict (nothing is pushed automatically; [Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)), and the time of the last sync. **Phasing:** Sync comes with the [Git epic](../../backlog/epics/git.md); until then the button shows, disabled.
- **`project.toml`**: a row with `<project>/project/project.toml · read-only` (Mono 12.5px `--tx3`). Below: "This file is config, committed in the project repo. Edit it in your editor." (`Change with Chief of Staff` comes later: [Project config with the Chief of Staff](../../backlog/cos-project-config.md).) Then the file's actual content in a selectable `<pre>`, exactly as on disk ([Project config](../models/project-config.md)).

Everything in this block is read-only. Changes to `project.toml` on disk show within about 1 s. An invalid file shows an error banner naming the field ([Project config](../models/project-config.md#invalid-config)).
