---
type: spec
status: built
tags: [feature, ui]
---

# Project page

The place where the CEO works: a project, its team at work, and everything about it, one click apart (the CEO, 2026-10-31: with Dashboard, Sessions and Reviews in the nav, then the projects, then the teams, moving between a session's reviews, board and settings was a maze). A project has one team at most ([Deployment](../models/deployment.md#invariants)), so its page is also that team's.

## Sections

The page has no header of its own: its sections are listed in the [sidebar](./app-shell.md#sidebar), under the project open, so the main area holds one row of tabs at most (the agent tabs, on Session). In order:

| Section | Path | What |
|---|---|---|
| **Session** | `session?agent=` | The team's terminals, an agent at a time ([Sessions](./sessions.md)) |
| **Inbox** | `inbox`, `inbox/<review>?since=1` | What waits on the CEO there, reviews first; a review opens inside it ([Reviews](./reviews.md)) |
| **Wiki** | `wiki?entry=` | The project's description, then its wiki ([Wiki reader](./wiki-reader.md)) |
| **Board** | `board` | Kanban boards over the wiki ([Board](./board.md)) |
| **Repos** | `repos`, `repos/<repo>?branch=` | The repos, added and managed; each opens its page, its branches, files and history graph ([Repos](./repos.md)) |
| **Deployment** | `deployment` | The team, its agents and their worktrees, the work in flight and merged; Deploy and Stop ([Project deployment](./project-deployment.md)) |
| **Activity** | `activity` | The team's messages and git actions, with the map of who calls whom ([Project activity](./project-activity.md)) |
| **Settings** | `settings` | The project's configuration ([Project settings](./project-settings.md)) |

Each path is under `/projects/<project>/`, and each section is its own route, so the URL is the view: Back, a reload and a shared link show the same section. **`/projects/<project>`** opens **Session** while a team is on the project, **Wiki** otherwise (the URL is replaced, so Back doesn't return to it). Switching projects starts each section fresh.

The names: **Inbox** (was Reviews, which also took the project's other requests from Overview), **Wiki** (was Overview), **Repos** (was Repositories), **Deployment** (replaced Worktrees). **Board** keeps its name: it is wikiview's `board`, one word for one board or several (the CEO, 2026-10-31).

## Layout

Every section has the same slim sides, **24px**, and starts 24px from the top (20px for the Wiki and a review's header), so moving between them nothing jumps. Text meant to be read keeps a comfortable measure: the wiki entry and the project's description at most 780px, the Inbox at most 920px; boards, repos, diffs and timelines use the width. Session fills the height, scrolling inside its terminal ([App shell → Layout](./app-shell.md#layout)).

## Links into it

Every link to a session (Dashboard, notifications, Deploy, Team page, Agent page, Deployment, the pipeline, Activity) opens the project's Session on that agent; every link to a review opens it in the project's Inbox. Under 760px the sidebar, and so the sections, is behind the menu button.
