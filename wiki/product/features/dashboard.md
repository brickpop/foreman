---
type: spec
status: partial
tags: [feature, ui, requests]
---

# Dashboard

The start page: what is at work, the projects and the teams, each a click from where it is handled. The screen at `/`, the sidebar's first entry; it was called Home until 2026-10-20, when "home" came to mean only Foreman's folder, `~/.foreman` ([Home](../models/home.md)) (the CEO).

It was the CEO's inbox until 2026-10-21, when the CEO took the reviews out of it: they are decided in [Reviews](./reviews.md), which has the sidebar's count, so a list of them here was the same thing twice. The Dashboard became a page to get started from, and to see the whole at a glance (the CEO, 2026-10-21).

Width up to 1080px, the page's 48px sides (16px under 760px), 40px between sections. Each section is a label (`SectionLabel`) and cards in a grid that fills the width (`auto-fill`), gap 12; a card is `--panel`, `--line` border, radius 12, padding `14px 16px`.

## Header

- Title (page H1): a greeting by the local time: `Good morning` (5:00–11:59), `Good afternoon` (12:00–17:59), `Good evening` (18:00–23:59), `Good night` (0:00–4:59).
- Subtitle: `<running> sessions working across <deployments> deployments.` Running = instances with status `running` ([Instance](../models/instance.md)); deployments = active [deployments](../models/deployment.md).
- On the right: **`New project`** (the [Add a project](./add-project.md) dialog) and a primary **`Deploy a team`** ([Deploy a team](./deploy-team.md), no team or project preset), disabled until there are a team and a project.

## Setup checklist

While any setup step is left, the setup checklist comes under the header: Podman, its machine (macOS), the agent image, each harness login, a team, a project, a code repo per project, each with its button ([First run](./first-run.md#the-checklist)). It goes away once every step is done.

## Needs you

The open [requests](../models/request.md) of every kind but reviews, across projects, one **request panel** each (gap 10); left out when there are none. Reviews are decided in [Reviews](./reviews.md) only. The kinds, their actions and their effect are in [Requests](./requests.md). The Dashboard and the project's Overview ([Needs you](./project-pipeline.md#1-needs-you)) use the same request panel component, so they can't drift: approvals and questions carry the free-text answer field here too.

- **Request panel**: `--panel`, radius 12, padding `18px 20px`, gap 14. The whole panel opens its agent's session ([Sessions](./sessions.md)); its buttons, links and answer field keep their own action, and selecting text opens nothing. A Foreman-raised request (no agent) opens nothing.
- **Header row**: Avatar 32 with an amber ring ([Avatar](../design/avatar.md)), the agent name (600; avatar and name are a button doing the same, the keyboard path), the context (13px `--tx4`): `<Team> on <project>`; a spacer; the kind chip.
- **Text**: the request summary, 16px, line-height 1.5.
- **Buttons** (wrap, gap 8): its options, styled by position ([Design system](../design/design-system.md#buttons)). No `Open session` button: the card itself opens the session. Choosing an option resolves the request ([Requests](./requests.md)).
- **Order**: blocked first, then approvals, questions and findings; within a kind, oldest first.

Today only review requests are built, so the section doesn't show yet.

## At work

One card per active deployment with an agent awake, by project then team (cards at least 320px wide), with `All sessions →` on the label's right (to [Sessions](./sessions.md)).

- **Header**: the team's name (600, a link to its deployment in Sessions), `on <project>` (13px `--tx4`), and on the right `<n> working` (`--ok`) or `idle` (`--tx4`). Under it the deployment's goal (13.5px `--tx3`, two lines at most).
- **A row per agent but the asleep ones** (not started yet, woken when needed: noise here; the CEO, 2026-10-21), the lead first, then the team file's order: Avatar 28 with its status dot, the name (500), the `lead` tag, the role (`--tx4`); under it, in Mono 11.5px `--tx4`, what its harness last said it was doing and when (`Editing api/cal.go · 14:05`), or its status. A row opens that agent's session.
- **Empty**: a dashed box: `No team is at work. Deploy one on a project, with a goal, and its sessions show here.` with a primary `Deploy a team`; with no team or no project yet, `No team is at work yet. Add a project and a team, then deploy the team on it.`

## Projects

One card per project (cards at least 240px wide), then a dashed **`+ New project`** card (`Your repos, any folder on this machine.`) opening Add a project. A card opens the project's Overview ([Wiki reader](./wiki-reader.md)).

- The name (600 15px), and on the right its state: a dot and `needs you` (`--acc`) if it has open requests (reviews included), `on track` (`--ok`) if any instance is running, otherwise `idle` (`--tx4`).
- Its description (13.5px `--tx3`, two lines, kept at two so cards line up), or `No description yet.`
- Its code repos as Mono chips (`--raised`), the first three then `+<n>`; a repo with a problem in `--err`, the problem as its tooltip; `No code repo yet` when it has none.
- `<Team>, <Team> · <n> agents working`, or `No team deployed` (12.5px `--tx4`).

## Teams

One card per team (at least 240px wide), then a dashed **`+ New team`** card (`The Chief of Staff drafts it with you.`) opening the [Chief of Staff](./chief-of-staff.md) to create a team. A card opens the team's page ([Team](./team-page.md)).

- The name (600 15px), its purpose (two lines, as a project's description), or `No purpose written yet.`
- Its members' avatars (26, overlapping), the lead first, at most six then `+<n>`, and `<n> agents`.
- `On <project>, <project>` or `Not deployed` (12.5px `--tx4`), and a ghost **`Deploy`** opening Deploy a team with this team preset (shown once there is a project).
