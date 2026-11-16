---
type: spec
status: partial
tags: [feature, ui, requests]
---

# Dashboard

The start page: the projects and the teams, each a click from where it is handled. The screen at `/`, opened by the sidebar's brand; it was called Home until 2026-10-20, when "home" came to mean only Foreman's folder, `~/.foreman` ([Home](../models/home.md)) (the CEO).

It was the CEO's inbox until 2026-10-21, when the CEO took the reviews out of it: they are decided in each project's [Inbox](./reviews.md#the-inbox), which has the sidebar's count, so a list of them here was the same thing twice. The Dashboard became a page to get started from, and to see the whole at a glance (the CEO, 2026-10-21). Its **At work** section (a card per active deployment) went on 2026-10-31, when the work moved into each project: a project card opens its team at work (the CEO).

Width up to 1080px, the page's 24px sides (16px under 760px), 40px between sections. Each section is a label (`SectionLabel`) and cards in a grid that fills the width (`auto-fill`), gap 12; a card is `--panel`, `--line` border, radius 12, padding `14px 16px`.

## Header

- Title (page H1): a greeting by the local time: `Good morning` (5:00–11:59), `Good afternoon` (12:00–17:59), `Good evening` (18:00–23:59), `Good night` (0:00–4:59).
- Subtitle: `<running> sessions working across <projects> projects.` Running = instances with status `running` ([Instance](../models/instance.md)); projects = those with a team on them (an active [deployment](../models/deployment.md) each).
- On the right: **`New project`** (the [Add a project](./add-project.md) dialog) and a primary **`Deploy a team`** ([Deploy a team](./deploy-team.md), no team or project preset), disabled until there are a team and a project.

## Setup checklist

While any setup step is left, the setup checklist comes under the header: Foreman's HQ (always done; Restore from another machine offered while pristine), Podman, its machine (macOS), the agent image, each harness login, a team, a project, a code repo per project, each with its button ([First run](./first-run.md#the-checklist)). It goes away once every step is done.

## projects.toml needs fixing

While `projects.toml` has problems ([Project config → projects.toml](../models/project-config.md#projectstoml)), a red notice under the header lists them: the projects show as last read and can't be changed until the CEO fixes the file (`GET /api/home`, `projectsProblems`).

## Not backed up

The HQs their origin doesn't back up, one callout each (`--attn`; `GET /api/home`, `backups`), after the setup checklist; left out when there are none (the CEO, 2026-11-16: a callout, its path and command copyable). Each names the HQ (Foreman's, or the project's), its path (Mono, truncated) with a button copying it, and the problem: **no origin**, followed in the same paragraph by "Make an empty repo for it (GitHub, GitLab…), then run this with its URL in place of `<its URL>`. Foreman pushes it at every start and stop from then on." and the server's `command`, one step per line (`cd "<path>"`, `git remote add origin <its URL>`, `git push -u origin main`), in a terminal-coloured block with a button copying it (Foreman's git config is the CEO's; Foreman has no field for it); or **not synced with its origin**, with git's reason and no command (a conflict, aborted; uncommitted changes in the way; a remote that refused or couldn't be reached). Judged at the start's [sync](../models/home.md#syncing-the-hqs), and for a project made since, by whether it has an origin. A remote added by hand shows at the next start.

## Not on this machine

The projects in `projects.toml` whose HQ isn't on this machine (`here: false`): another machine's, after a [restore](../models/home.md#a-new-machine) or a sync brought their records in, or one whose HQ was deleted here. They are nowhere else in the app (no sidebar row, no card, not deployable), hidden ones included; this section, after the Projects grid and before Teams, lists them by name, left out when there are none. One row each: the name, the HQ's remote (Mono), and **Bring here** (`POST /api/projects/:id/bring`), which clones the HQ into its place, then opens the project's [Repos](./repos.md), where each code repo is located or cloned (**Clone it**). One with no remote recorded (its HQ had no origin where it was made) says so instead of the button: `Its HQ has no origin: push it where it is, then start Foreman there once`. A refused clone shows git's reason under the row.

## Needs you

The open [requests](../models/request.md) of every kind but reviews, across projects, one **request panel** each (gap 10); left out when there are none. Reviews are decided in each project's [Inbox](./reviews.md#the-inbox) only. The kinds, their actions and their effect are in [Requests](./requests.md). The Dashboard and the project's Inbox ([Needs you](./project-pipeline.md#1-needs-you)) use the same request panel component, so they can't drift: approvals and questions carry the free-text answer field here too.

- **Request panel**: `--panel`, radius 12, padding `18px 20px`, gap 14. The whole panel opens its agent's session (its project's [Session](./sessions.md)); its buttons, links and answer field keep their own action, and selecting text opens nothing. A Foreman-raised request (no agent) opens nothing.
- **Header row**: the agent name (600; a button opening its session, `--accTx` on hover), the context (13px `--tx4`): `<Team> on <project>`; a spacer; the kind chip.
- **Text**: the request summary, 16px, line-height 1.5.
- **Buttons** (wrap, gap 8): its options, styled by position ([Design system](../design/design-system.md#buttons)). No `Open session` button: the card itself opens the session. Choosing an option resolves the request ([Requests](./requests.md)).
- **Order**: blocked first, then approvals, questions and findings; within a kind, oldest first.

Today only review requests are built, so the section doesn't show yet.

## Projects

One card per project on this machine not [hidden](../models/project-config.md#the-project-record) (cards at least 240px wide), by name, as the [sidebar](./app-shell.md#sidebar) orders them, then a dashed **`+ New project`** card (`Your repos, any folder on this machine.`) opening Add a project. A card opens the [project](./project-page.md): its Session while a team is on it, its Wiki otherwise (the CEO, 2026-10-31). **Loading**: a card-sized [loading placeholder](../design/design-system.md#surfaces) at the head of Projects and of Teams until they are in.

- A [card](../design/design-system.md#surfaces). In its header band, the name, and on the right its state as a status pill: `needs you` (`--attn`) if it has open requests (reviews included), `on track` (`--ok`) if any instance is running, otherwise `idle` (`--tx4`).
- Its description (13.5px `--tx3`, two lines, kept at two so cards line up), or `No description yet.`
- Its code repos as Mono chips (`--raised`), the first three then `+<n>`; a repo with a problem in `--err`, the problem as its tooltip; `No code repo` (`--tx4`) when it has none.
- `<Team> · <n> agents working` (one team at most), or `No team deployed` (12.5px `--tx4`).

## Teams

One card per team (at least 240px wide), then a dashed **`+ New team`** card (`The Chief of Staff drafts it with you.`) opening the [Chief of Staff](./chief-of-staff.md) to create a team. A card opens the team's page ([Team](./team-page.md)). [Settings → Teams](./settings.md#teams) shows the same cards.

- A [card](../design/design-system.md#surfaces). In its header band, the name, and on the right a status pill: `deployed` (`--ok`) or `not deployed` (`--tx4`).
- Its purpose (two lines, as a project's description), or `No purpose written yet.`
- `<n> agents: <names>` (12.5px `--tx4`, the lead first, cut with an ellipsis, all of them in its title).
- `On <project>, <project>` when deployed (12.5px `--tx4`), and a ghost **`Deploy`** opening Deploy a team with this team preset (shown once there is a project).
