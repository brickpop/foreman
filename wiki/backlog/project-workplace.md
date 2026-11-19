---
type: task
status: in-progress
priority: high
tags: [feature, ui]
epic: workflow
---

# The project is the workplace: one team per project, its sections in the sidebar

**Why:** with Dashboard, Sessions and Reviews in the nav, then the projects, then the teams, moving around a session's reviews, board and settings meant jumping between screens, and the CEO got lost (the CEO, 2026-10-31). The work happens on a project, so the project's page becomes the place to work: its session, its inbox, its wiki, its board, all one click apart, without two rows of tabs. It supersedes [Sessions: a list of deployments, and a page per deployment](./archive/sessions-by-project.md).

## Decided (the CEO, 2026-10-31)

- **One team per project, at most.** Deploying another team on a project replaces the one there: it is stopped as by Stop (sessions end, worktrees and branches kept, open requests expire), then the new one deploys. The Deploy dialog says so first. A team still works on many projects at once, independently. Not shipped yet: no migration of existing state, the CEO adjusts locally.
- **Sessions and Reviews leave the nav.** The sidebar: Dashboard, the projects, the teams, Settings. **Only the project open** expands its sections under its row: **Session · Inbox · Wiki · Board · Repos · Deployment · Activity · Settings**. The Archived group stays. So the page has one row of tabs at most: the agent tabs, on Session.
- **A project opens on Session** when a team is on it, on **Wiki** otherwise.
- **Session**: the deployment's page as it was, its header slimmed (no Team / Reviews / Board links, no Deployment menu: the sidebar is the way around). No team: an empty state with Deploy.
- **Inbox** (was Reviews, now per project): the reviews needing the CEO, the project's other requests (Needs you, from Overview), what waits on the lead, the last done. A review opens inside it (`← Inbox`). Merge, Request changes and Dismiss go back to the Inbox; the Merge toast opens the session.
- **Wiki** (was Overview): the wiki reader; the project's path and description head it (the big project header goes).
- **Board** keeps its name (it is WikiShell's `board`, one word for one or several). **Repos** (was Repositories).
- **Deployment** (replaces Worktrees): the team on the project, its members (each opening its session), their worktrees, the work in flight and merged; Stop, and Deploy (replacing). Project settings lose their "Teams on this project" section to it.
- **Activity**: the session's Activity panel goes; the section holds it all: the timeline on the left, the "who calls whom" map on the right filtering it, older sessions below.
- **Dashboard**: At work goes. A card per project (its team and state; opens Session, or Wiki without a team), then a card per team.
- **Compact and consistent**: the same slim padding on every section; reading text (wiki, review comments, empty states) keeps a comfortable max width.
- **URLs are paths**: `/projects/<id>/session?agent=`, `/inbox`, `/inbox/<review>?since=1`, `/wiki?entry=`, `/board`, `/repos?repo=&branch=`, `/deployment`, `/activity`, `/settings`. `/sessions` and `/reviews` go, no redirects.

## Steps

- [x] server: Deploy replaces another team's active deployment on the project (stopped first, serialized); a partial unique index, one active deployment per project; tests
- [x] specs: Deployment, Deploy a team, App shell, a Project page spec (sections, URLs, default), Sessions, Reviews, Wiki reader, Worktrees → Deployment, Project activity, Repositories → Repos, Project settings, Dashboard, Team page, Notifications, journey, use cases, glossary
- [x] router: the project's layout route and its section routes; `/sessions`, `/reviews` gone
- [x] sidebar: the open project's sections, the Inbox count
- [x] Session, Inbox (+ review), Wiki, Repos, Deployment, Activity sections; every link to a session or review rewritten
- [x] Dashboard: project cards, team cards; `DeploymentCard`, `sessionGroups`, the Sessions list code removed
- [x] Deploy dialog: the replacement warning; Stop dialog's links
- [x] tests: the section routes and defaults, the sidebar model, the Deploy replacement wording, the activity filter over pages
- [x] the goal is the project's (the CEO, 2026-10-31): the "Current goal" line of its wiki's `backlog/index.md`, kept by the lead, pointed to by every session prompt; no goal on deploy, no opening message (the CEO writes first; the lead asks for the goal when there is none); the deployment's `goal` column dropped (migration 011)
- [ ] tried by the CEO in the browser

Known edges left: [Technical debt → One team per project](../technical-debt.md#one-team-per-project-known-edges). Open for the CEO: the journey and use cases still tell two teams on one project (Vesper beside Atlas on ledger).
