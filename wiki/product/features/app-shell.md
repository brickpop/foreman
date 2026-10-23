---
type: spec
status: planned
tags: [feature, ui]
---

# App shell

The frame every screen lives in: a fixed left sidebar, a main area, and the overlays (drawers, side sheet, modals, toasts). Styles come from the [Design system](../design/design-system.md); the visual reference is the [prototype](../design/prototype/foreman-prototype.dc.html).

## Layout

A two-column grid: sidebar **220px**, main area `minmax(0, 1fr)`, full viewport height. The main area scrolls as a whole, except on [Sessions](./sessions.md), which fills the height and scrolls inside its panes.

**One breakpoint** (basic responsiveness): under ~760px the sidebar collapses behind a menu button at the top left and opens as an overlay; everything else keeps its layout and scrolls horizontally where it needs the width (e.g. the 880px [board](./board.md)).

Screens: [Dashboard](./dashboard.md), [Sessions](./sessions.md), [Reviews](./reviews.md) (the list and a review's page), Project ([Wiki reader](./wiki-reader.md) with the [project pipeline](./project-pipeline.md)'s Needs you, [Board](./board.md), [Repositories](./repositories.md), [Worktrees](./worktrees.md), [Activity](./project-activity.md), [Project settings](./project-settings.md)), [Team](./team-page.md), [Agent](./agent-page.md), [Chief of Staff](./chief-of-staff.md#the-page), [Settings](./settings.md). The app opens on the Dashboard (or the last screen, see [UI state](../models/ui-state.md)).

## Sidebar

`--side` background, `--line` right border. Top to bottom:

1. **Brand** (padding `20px 18px 16px`, gap 9): a 22×22 amber tile (radius 5) with "F" in `--accInk` (Mono 700 12px), and "Foreman" (600 15px).
2. **Scrolling area** (padding `0 10px 12px`, gap 18 between groups):
   - **Nav**: `Dashboard`, `Sessions`, and `Reviews`, with a count badge of the reviews that need the CEO (their review requests open; [Reviews](./reviews.md)): the nav's only count (the CEO, 2026-10-21). Items: padding `8px 10px`, radius 6, 500. Active: `--raised`, `--tx`. Inactive: transparent, `--tx2`.
   - **Projects** label, then one row per project not archived (padding `6px 10px`, 13.5px). A project that needs the CEO shows a 7px amber dot. Active while that project's screen is open. Then **`+ New project`** (13px `--tx4`), which opens [Add a project](./add-project.md). Then, when any project is [archived](../models/project-config.md#the-project-record), a collapsed **`Archived · N`** row (13px `--tx4`, a chevron) that opens to their rows, the same as the others; it shows open while one of them is the open project.
   - **Teams** label, then one row per team with, right-aligned (12px `--tx4`), where it is deployed: the project id if one deployment, `N projects` if more, `—` if none. A team row is active on its Team page and on the Agent page of any of its members. Then **`+ New team`**, which opens the [Chief of Staff](./chief-of-staff.md) "Create a team" flow.
   - Section labels are plain labels: not links, not collapsible. There is no Teams index screen (the prototype has one that nothing links to): the sidebar already lists every team.
3. **Footer** (padding `12px 10px`, top border):
   - **Settings row**: "Settings" with the listen address on the right (e.g. `localhost:8888`, Mono 11.5px `--tx4`). Opens [Settings](./settings.md).

Data: projects and teams from the [home](../models/home.md) (the projects' records and project repos, the global repo's team files), deployments from the runtime state ([Deployment](../models/deployment.md)); the badge and dots from open [requests](../models/request.md). All of it updates live from server events ([API](../architecture/api.md)).

## Overlays

| Overlay | Geometry | Layer |
|---|---|---|
| Side sheet (board card) | Fixed right, `min(460px, 100vw)` wide, full height, `--panel`, `--line2` left border | 44 |
| Modals ([Deploy](./deploy-team.md), [Add project](./add-project.md)) | Centered panel `min(520px, 100%)`, at most the window's height less the backdrop padding (it scrolls past that), padding 24, radius 14, on a `rgba(5,6,7,0.55)` backdrop with 20px padding | 50 |
| Toasts | Fixed top 20px, right 20px, 320px wide, gap 8 | 60 |

Drawers and the side sheet overlay the content; they don't push it. Clicking outside a drawer or the side sheet closes it (except on another board card, which the side sheet then shows: [Board](./board.md#interactions)), and clicking a modal backdrop closes the modal. Escape closes the topmost overlay, and modals trap focus. The exception is a modal holding a terminal (the Connect dialog, [Settings](./settings.md#profiles)): it closes through its own buttons only, since Escape belongs to the terminal ([Design system](../design/design-system.md#accessibility-v1-scope)).

## Toasts

In-app mirrors of [notifications](./notifications.md), shown regardless of browser permission.

- Toast panel: `--panel`, 1px `--line3`, radius 10, padding `12px 14px`, `--shadow`, pointer cursor. An 8px colored dot (top margin 6px), a title (600 13.5px) and a body (13px `--tx2`, line-height 1.4).
- Dismissed automatically after **6.5 s**, on click, or with its **×** (top right, `--tx4`), which only dismisses it. A click on a toast tied to an instance opens that session in [Sessions](./sessions.md); every other toast navigates to the screen it is about (a board move opens the entry in the [Wiki reader](./wiki-reader.md), "project added" opens the project), so every toast click goes somewhere; the × is the way to dismiss without leaving the screen.

| Event | Title | Body | Dot |
|---|---|---|---|
| Instance done | `<Name> is done on <project>` | the handoff summary, e.g. `Idempotency keys handed off to Mara.` | `--info` |
| New request | `<Name> needs you on <project>` | the request summary | `--acc` |
| Deployed | `<Team> deployed to <project>` | `<N> new sessions starting.` (opens the lead's session) | `--ok` |
| Stopped | `<Team> stopped on <project>` | `Worktrees are kept. Other deployments keep running.` | `--tx4` |
| Project created | `<id> created` | `Made its project repo at <path>, with project.toml and a starter wiki.`, `Cloned its project repo into <path>.`, or `Took over the project repo left at <path>, as it was.` | `--ok` |
| Project removed | `<id> removed` | none | `--tx4` |
| Stopped by Foreman (its project is gone) | `Foreman stopped <Team> on <project>` | the reason, e.g. `ledger is gone: all its code repos are missing` (opens the Dashboard) | `--err` |
| Config review resolved | `project.toml` | `Config change merged to main.` (`--ok`) or `Config change rejected.` (`--tx4`) | |
| Board move | `<path>` (e.g. `backlog/LED-44.md`) | `status: <s>`, plus ` · priority: <lane>` when the lane changed | `--info` |
| Notifications enabled | `Notifications on` | `You’ll hear from agents when they finish or need you.` | `--ok` |
