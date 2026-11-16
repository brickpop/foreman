---
type: spec
status: planned
tags: [feature, ui]
---

# App shell

The frame every screen lives in: a fixed left sidebar, a main area, and the overlays (drawers, side sheet, modals, toasts). Styles come from the [Design system](../design/design-system.md); the visual reference is the [prototype](../design/prototype/foreman-prototype.dc.html).

## Layout

A two-column grid: sidebar **220px**, main area `minmax(0, 1fr)`, full viewport height. The main area scrolls as a whole, except on a project's [Session](./sessions.md), which fills the height and scrolls inside its terminal. Every screen has the same slim sides, 24px ([Project page → Layout](./project-page.md#layout)).

**One breakpoint** (basic responsiveness): under ~760px the sidebar collapses behind a menu button at the top left and opens as an overlay; everything else keeps its layout and scrolls horizontally where it needs the width (e.g. the 880px [board](./board.md)).

Screens: [Dashboard](./dashboard.md); a [Project](./project-page.md), with its sections [Session](./sessions.md), [Inbox](./reviews.md#the-inbox) (a review's page inside it), [Repos](./repos.md), [Board](./board.md), [Wiki](./wiki-reader.md), [Team](./project-team.md) (with its [Activity](./project-activity.md)) and [Settings](./project-settings.md); [Team](./team-page.md), [Agent](./agent-page.md), [Chief of Staff](./chief-of-staff.md#the-page), [Settings](./settings.md). The app opens on the Dashboard (or the last screen, see [UI state](../models/ui-state.md)). Sessions and Reviews left the nav on 2026-10-31: the work happens inside a project (the CEO).

## Sidebar

`--side` background, `--line` right border. Top to bottom:

1. **Brand**, the way to the **Dashboard** (the CEO, 2026-11-02: no nav entry for it): a link (margin `12px 10px 8px`, padding `8px`, radius 6, `--hover` behind on hover, gap 9, titled `Dashboard`) holding a 22×22 accent tile (`--acc`, radius 5) with "F" in `--accInk` (Mono 700 12px), and "Foreman" (600 15px).
2. **Scrolling area** (padding `0 10px 12px`), with no label: it holds only the projects (the CEO, 2026-11-02):
   - One row per project on this machine not hidden (one whose HQ isn't here is only on the [Dashboard](./dashboard.md#not-on-this-machine)), by the name it shows (its display name, else its id readable: [Project config](../models/project-config.md#the-project-record)) (padding `6px 10px`, 13.5px). **By name** (the CEO, 2026-11-04, replacing latest first of 2026-11-01): as people sort words, case and accents aside, numbers by value (`byName`). A project row, folded (every project but the open one, and the open one when folded), shows **one 7px dot** (the CEO, 2026-11-06): the attention dot (`--attn`, `Needs you`) while it has open requests (reviews included), else its Session's status dot while its team is busy, else none. Unfolded, the row shows none: the status dot is on **Session**, the requests are **Inbox**'s count. **The project open** (the one the CEO is in, not the others; the CEO, 2026-10-31) is in `--tx` 500 with a chevron, and lists its [sections](./project-page.md#sections) under it (indented 14px, a `--line2` left rule; rows padding `4px 10px`, 13px `--tx3`, the active one `--raised` `--tx`), **Session** with its team's status dot while the team is busy (its liveliest agent awaiting the CEO, running, starting or queued; none at rest, idle, asleep or stopped: the CEO, 2026-11-02), **Inbox** with a count badge of the project's open requests: the sections live here, so the main area needs no second row of tabs. A click on the open project's name folds or unfolds its sections, staying on the page (folded, it shows its row dot); on another project it opens that one, unfolded (the CEO, 2026-11-01). Then, when any project is [hidden](../models/project-config.md#the-project-record), a collapsed **`More`** row (13px `--tx4`, a chevron; `Archived` until 2026-11-13) that opens to their rows, the same as the others; it shows open while one of them is the open project. Folded, it shows one dot for them all (the CEO, 2026-11-13: an agent waiting there isn't missed): the attention dot while any of them has open requests, else the first busy one's status dot, else none. Then **`+ New project`** (13px `--tx4`), which opens [Add a project](./add-project.md).
   - **Only the projects** (the CEO, 2026-11-02: what the CEO uses day to day): the teams moved to [Settings → Teams](./settings.md#teams), with their pages.
3. **Footer** (padding `12px 10px`, top border):
   - **Settings row**: "Settings" (the address it once showed, `localhost:<port>`, went on 2026-11-02: the browser's address bar has it). Opens [Settings](./settings.md); active on every Settings page, the team and agent pages included.

Data: projects and teams from the [home](../models/home.md) (the projects' records and HQs, the team files in Foreman's HQ), deployments from the runtime state ([Deployment](../models/deployment.md)); the Inbox badge and the dots from open [requests](../models/request.md). All of it updates live from server events ([API](../architecture/api.md)).

## Overlays

| Overlay | Geometry | Layer |
|---|---|---|
| Side sheet (board card) | Fixed right, `min(460px, 100vw)` wide, full height, `--panel`, `--line2` left border | 44 |
| Modals ([Deploy](./deploy-team.md), [Add project](./add-project.md)) | Centered panel `min(520px, 100%)`, at most the window's height less the backdrop padding (it scrolls past that), padding 24, radius 14, on a `rgba(5,6,7,0.55)` backdrop with 20px padding; its title with a close `×` at the right (the CEO, 2026-11-04), as drawers have | 50 |
| Toasts | Fixed top 20px, right 20px, 320px wide, gap 8 | 60 |

Drawers and the side sheet overlay the content; they don't push it. Clicking outside a drawer or the side sheet closes it (except on another board card, which the side sheet then shows: [Board](./board.md#interactions)), and clicking a modal backdrop closes the modal. Escape closes the topmost overlay, and modals trap focus. The exception is a modal holding a terminal (the Connect dialog, [Settings](./settings.md#profiles)): it closes through its own buttons only, since Escape belongs to the terminal ([Design system](../design/design-system.md#accessibility-v1-scope)).

## Toasts

In-app mirrors of [notifications](./notifications.md), shown regardless of browser permission.

- Toast panel: `--panel`, 1px `--line3`, radius 10, padding `12px 14px`, `--shadow`, pointer cursor. An 8px colored dot (top margin 6px), a title (600 13.5px) and a body (13px `--tx2`, line-height 1.4).
- Dismissed automatically after **6.5 s**, on click, or with its **×** (top right, `--tx4`), which only dismisses it. A click on a toast tied to an instance opens that agent in its project's [Session](./sessions.md), one about a review opens it in the project's Inbox; every other toast navigates to the screen it is about (a board move opens the entry in the [Wiki reader](./wiki-reader.md), "project added" opens the project), so every toast click goes somewhere; the × is the way to dismiss without leaving the screen.

| Event | Title | Body | Dot |
|---|---|---|---|
| Instance done | `<Name> is done on <project>` | the handoff summary, e.g. `Idempotency keys handed off to Mara.` | `--info` |
| New request | `<Name> needs you on <project>` | the request summary | `--attn` |
| Deployed | `<Team> deployed to <project>` | `<N> new sessions starting.` (opens the lead's session) | `--ok` |
| Stopped | `<Team> stopped on <project>` | `Worktrees are kept. Other deployments keep running.` (opens the project's Team) | `--tx4` |
| Project created | `<id> created` | `Made its HQ at <path>, with project.toml and a starter wiki.`, `Cloned its HQ into <path>.`, or `Took over the HQ left at <path>, as it was.` | `--ok` |
| Project removed | `<id> removed` | none | `--tx4` |
| Stopped by Foreman (its project is gone) | `Foreman stopped <Team> on <project>` | the reason, e.g. `ledger is gone: all its code repos are missing` (opens the Dashboard) | `--err` |
| Config review resolved | `project.toml` | `Config change merged to main.` (`--ok`) or `Config change rejected.` (`--tx4`) | |
| Board move | `<path>` (e.g. `backlog/LED-44.md`) | `status: <s>`, plus ` · priority: <lane>` when the lane changed | `--info` |
| Notifications enabled | `Notifications on` | `You’ll hear from agents when they finish or need you.` | `--ok` |
