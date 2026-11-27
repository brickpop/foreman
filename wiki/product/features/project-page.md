---
type: spec
status: built
tags: [feature, ui]
---

# Project page

The place where the CEO works: a project, its team at work, and everything about it, one click apart (the CEO, 2026-10-31: with Dashboard, Sessions and Reviews in the nav, then the projects, then the teams, moving between a session's reviews, board and settings was a maze). A project has one team at most ([Deployment](../models/deployment.md#invariants)), so its page is also that team's.

The window's title names the project, first so a narrow tab still shows it: `<Project> · Foreman` (its readable name; the CEO, 2026-11-04). Every other page is `Foreman`.

## Sections

Its sections are the [rail](./app-shell.md#the-rail)'s icons while the project is open, so the main area holds one row of tabs at most (the agent tabs, on Session). In order (the design the CEO accepted on 2026-11-28 puts Repos and Wiki side by side):

| Section | Path | What |
|---|---|---|
| **Session** | `session?agent=` | The team's terminals, an agent at a time ([Sessions](./sessions.md)) |
| **Inbox** | `inbox`, `inbox/<review>?since=1` | What waits on the CEO there, reviews first; a review opens inside it ([Reviews](./reviews.md)) |
| **Board** | `board` | Kanban boards over the wiki ([Board](./board.md)) |
| **Repos** | `repos`, `repos/<repo>?branch=` | The repos, added and managed; each opens its page, its branches, files and history graph ([Repos](./repos.md)) |
| **Wiki** | `wiki?entry=` | The project's description, then its wiki ([Wiki reader](./wiki-reader.md)) |
| **Team** | `team?view=` | The team at work: its agents and their worktrees, its messages and git actions, the work in flight and merged; Deploy and Stop ([Project team](./project-team.md)). Deployment and Activity until 2026-11-03: `/deployment` and `/activity` redirect here |
| **Settings** | `settings` | The project's configuration ([Project settings](./project-settings.md)) |

Each path is under `/projects/<project>/`, and each section is its own route, so the URL is the view: Back, a reload and a shared link show the same section. **`/projects/<project>`** opens **Session** while a team is on the project, **Wiki** otherwise (the URL is replaced, so Back doesn't return to it). Switching projects starts each section fresh.

The names: **Inbox** (was Reviews, which also took the project's other requests from Overview), **Wiki** (was Overview), **Repos** (was Repositories), **Team** (replaced Deployment, which replaced Worktrees, and Activity). **Board** keeps its name: it is WikiShell's `board`, one word for one board or several (the CEO, 2026-10-31).

## The project's bar

Every section starts with one bar naming the project, then the section: `<project> · Inbox` (the project's shown name 600 15px, `· <Section>` `--tx3`; padding `10px 16px`, a bottom border), the same look as the [Session](./sessions.md)'s header, which starts with the project's name too (the design the CEO accepted on 2026-11-28: every project page header starts with the project name). Session with a team on it has only its own header; without one it has the bar. A page inside a section (a repo, a review) keeps its own title under the bar, and the bar names its section (`Repos`, `Inbox`). Nothing else goes in the bar.

## Layout

Every section has the same slim sides, **24px**, and starts 24px under the project's bar (20px for a review's header), so moving between them nothing jumps. Text meant to be read keeps a comfortable measure: the wiki entry and the project's description at most 780px, the Inbox at most 920px; boards, repos, diffs and timelines use the width. Session fills the height, scrolling inside its terminal ([App shell → Layout](./app-shell.md#layout)).

## Links into it

Every link to a session (Dashboard, notifications, Deploy, Team page, Agent page, Deployment, the pipeline, Activity) opens the project's Session on that agent; every link to a review opens it in the project's Inbox. Under 760px the rail, and so the sections, is behind the menu button.
