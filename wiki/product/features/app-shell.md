---
type: spec
status: planned
tags: [feature, ui]
---

# App shell

The frame every screen lives in: a fixed left rail, a main area, and the overlays (drawers, side sheet, modals, toasts). Styles come from the [Design system](../design/design-system.md); the rail follows the design *Compact sidebar and session header*, accepted by the CEO on 2026-11-28.

## Layout

A two-column grid: the rail **44px**, main area `minmax(0, 1fr)`, full viewport height. The main area scrolls as a whole, except on a project's [Session](./sessions.md), which fills the height and scrolls inside its terminal. Every screen has the same slim sides, 24px ([Project page → Layout](./project-page.md#layout)).

**One breakpoint** (basic responsiveness): under 760px the rail hides behind a menu button (three lines) at the top left and opens as a 260px overlay; everything else keeps its layout and scrolls horizontally where it needs the width (e.g. the 880px [board](./board.md)). Touch is not a goal (the CEO, 2026-11-28).

Screens: [Dashboard](./dashboard.md); a [Project](./project-page.md), with its sections [Session](./sessions.md), [Inbox](./reviews.md#the-inbox) (a review's page inside it), [Board](./board.md), [Repos](./repos.md), [Wiki](./wiki-reader.md), [Team](./project-team.md) (with its [Activity](./project-activity.md)) and [Settings](./project-settings.md); [Team](./team-page.md), [Agent](./agent-page.md), [Chief of Staff](./chief-of-staff.md#the-page), [Settings](./settings.md). The app opens on the Dashboard (or the last screen, see [UI state](../models/ui-state.md)).

## The rail

One column, `--side` background, `--line` right border, each icon with one meaning always (the CEO, 2026-11-28: one column only). Top to bottom:

1. **The Foreman tile**: a 28px accent square with "F" (`--acc`, `--accInk`, Mono 700 12px). A click always goes to the **Dashboard**, where the tile shows raised (`--raised`), as an open section does; a ring shows only for keyboard focus. Its badge sits on its top right, inside the rail. Resting on it opens the [project popover](#the-project-popover).
2. **The open project's sections**, one icon each (18px, in a 36px row, `--tx3`; the open one `--raised` `--tx`), with a tooltip and an `aria-label` of its name: **Session** (a terminal window), **Inbox** (a tray), **Board** (columns), **Repos** (a fork), **Wiki** (a book), **Team** (two people), **Project settings** (sliders). Off a project (the Dashboard, Settings, the Chief of Staff) there are none: only the tile and the gear.
3. **Foreman settings** at the bottom, a gear: opens [Settings](./settings.md), and shows active on every Settings page, the team and agent pages included. Sliders and gear never look alike.

The icons are drawn by hand in `ui/src/components/icons.tsx`, like the others: no icon library.

**Which project is open:** every project page starts with [the project's bar](./project-page.md#the-projects-bar), `<project> · <Section>`; the project popover marks it, and the browser tab says `<project> · Foreman`.

### Badges

One mark per item: a **needs-you badge** (`--attn`, `--attnInk`, Mono 600 10px, radius 9, capped at `9+`) with the count of open [requests](../models/request.md) (reviews included), else a **busy dot**: the status colour of the team's liveliest agent while it awaits the CEO, runs, starts or is queued (none at rest, idle, asleep or stopped).

| Where | Shows |
|---|---|
| Foreman tile | The badge with what needs the CEO in every project but the open one (on the Dashboard: all of them, hidden ones included) |
| Each project in the popover | Its badge, else its busy dot, else nothing |
| Folded **More** | One mark for the hidden projects: their badge total, else the first busy one's dot |
| Inbox icon | The open project's badge |
| Session icon | The open project's busy dot |

Each mark has a tooltip saying what it means: the status, or `3 requests need you`. Screen readers hear the mark in the item's name: `Foreman, Dashboard. 3 requests need you in other projects`, `Inbox. 2 requests need you`, `Session. running`.

### The project popover

Beside the rail, at the tile's top, with no gap (`--panel`, `--line2` border, radius 8):

- **Dashboard**, then the projects on this machine not [hidden](../models/project-config.md#the-project-record), **by name** (as people sort words, case and accents aside, numbers by value: `byName`), each with its mark. The open project has a 3px accent bar and `(open)`. A click on a project opens its **Session**.
- **More (N)**: the hidden projects, folded, with one mark while folded; open while one of them is the open project.
- A divider, then **`+ New project`**, which opens [Add a project](./add-project.md).

**Opening and closing** (the same code and timings as the Session's [branch menu](./sessions.md#agent-tabs), `useHoverMenu`): it opens after the pointer rests **300ms** on the tile, so passing over it opens nothing, and closes **200ms** after the pointer leaves both the tile and the popover, when an item is picked, or on Escape. Opened by the pointer, it leaves the keyboard where it was (the terminal, say).

**Keyboard:** focusing the tile opens the popover at once, the focus staying on the tile: **Enter** still goes to the Dashboard, **↓** moves into the list, at the open project. In the list, ↑/↓ move, typing a letter jumps to a project, Enter opens it, Escape closes the popover and puts the focus back on the tile.

**Shortcuts**, caught by the window before the terminal (as [Line's](./sessions.md) is), by key position so any layout works:

- **`Ctrl+Alt+P`** opens the popover from anywhere, the focus on the open project. `Ctrl+K` is not used: shells use it.
- **`Ctrl+Alt+↓` / `Ctrl+Alt+↑`** moves to the next or previous visible project by name, round the ends, keeping the section (a repo's or a review's page opens that section's top, the Repos list or the Inbox). Hidden projects are skipped; from one of them, or from no project, it goes to the first visible one.

**Narrow screens:** in the 260px overlay the project list shows inline under the tile (no hovering), the icons show their names with each badge or dot at the row's right end, and picking anything closes the overlay.

Data: projects from the [home](../models/home.md), instances from the runtime state ([Deployment](../models/deployment.md)), the badges from open [requests](../models/request.md). All of it updates live from server events ([API](../architecture/api.md)).

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
