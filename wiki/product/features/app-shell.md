---
type: spec
status: planned
tags: [feature, ui]
---

# App shell

The frame every screen lives in: a fixed left sidebar, a main area, and the overlays (drawers, side sheet, modals, toasts). Styles come from the [Design system](../design/design-system.md); the visual reference is the [prototype](../design/prototype/foreman-prototype.dc.html).

## Layout

A two-column grid: sidebar **220px**, main area `minmax(0, 1fr)`, full viewport height. The main area scrolls as a whole, except on [Sessions](./sessions.md), which fills the height and scrolls inside its panes.

Screens: [Home](./home.md), [Sessions](./sessions.md), Project ([Wiki reader](./wiki-reader.md), [Changes](./changes.md), [Board](./board.md), [Project settings](./project-settings.md)), [Team](./team-page.md), [Agent](./agent-page.md), [Settings](./settings.md). The app opens on Home (or the last screen, see [UI state](../models/ui-state.md)).

## Sidebar

`--side` background, `--line` right border. Top to bottom:

1. **Brand** (padding `20px 18px 16px`, gap 9): a 22×22 amber tile (radius 5) with "F" in `--accInk` (Mono 700 12px), and "Foreman" (600 15px).
2. **Scrolling area** (padding `0 10px 12px`, gap 18 between groups):
   - **Nav**: `Home`, with a count badge of open requests when there are any ([Requests](./requests.md)), and `Sessions`. Items: padding `8px 10px`, radius 6, 500. Active: `--raised`, `--tx`. Inactive: transparent, `--tx2`.
   - **Projects** label, then one row per project (padding `6px 10px`, 13.5px). A project that needs the CEO shows a 7px amber dot. Active while that project's screen is open. Then **`+ New project`** (13px `--tx4`), which opens [Add a project](./add-project.md).
   - **Teams** label, then one row per team with, right-aligned (12px `--tx4`), where it is deployed: the project id if one deployment, `N projects` if more, `—` if none. A team row is active on its Team page and on the Agent page of any of its members. Then **`+ New team`**, which opens the [Chief of Staff](./chief-of-staff.md) "Create a team" flow.
   - Section labels are plain labels: not links, not collapsible.
3. **Footer** (padding `12px 10px`, top border):
   - **Chief of Staff card**: padding `9px 10px`, radius 8, `--accLine` border, `--accSoft` background. A 24×24 amber "C" tile (radius 6), "Chief of Staff" (600 13px) and "Hire, set up, anything else" (11.5px `--tx3`). Click opens the CoS drawer with no flow.
   - **Settings row**: "Settings" with the listen address on the right (e.g. `localhost:7420`, Mono 11.5px `--tx4`). Opens [Settings](./settings.md).

Data: projects, teams and deployments from the workspace ([Workspace](../models/workspace.md), [Deployment](../models/deployment.md)); the badge and dots from open [requests](../models/request.md). All of it updates live from server events ([API](../architecture/api.md)).

## Overlays

| Overlay | Geometry | Layer |
|---|---|---|
| Side sheet (board card) | Fixed right, `min(460px, 100vw)` wide, full height, `--panel`, `--line2` left border | 44 |
| Chief of Staff drawer | Fixed right, `min(420px, 100vw)` wide | 45 |
| Modals ([Deploy](./deploy-team.md), [Add project](./add-project.md)) | Centered card `min(520px, 100%)`, padding 24, radius 14, on a `rgba(5,6,7,0.55)` backdrop with 20px padding | 50 |
| Toasts | Fixed top 20px, right 20px, 320px wide, gap 8 | 60 |

Drawers and the side sheet overlay the content; they don't push it. Clicking a modal backdrop closes the modal. Escape closes the topmost overlay, and modals trap focus ([Design system](../design/design-system.md#accessibility-required-beyond-the-prototype)).

## Toasts

In-app mirrors of [notifications](./notifications.md), shown regardless of browser permission.

- Card: `--panel`, 1px `--line3`, radius 10, padding `12px 14px`, `--shadow`, pointer cursor. An 8px colored dot (top margin 6px), a title (600 13.5px) and a body (13px `--tx2`, line-height 1.4).
- Dismissed automatically after **6.5 s**, or on click. A click on a toast tied to an instance opens that session in [Sessions](./sessions.md).

| Event | Title | Body | Dot |
|---|---|---|---|
| Instance done | `<Name> is done on <project>` | the handoff summary, e.g. `Idempotency keys handed off to Mara.` | `--info` |
| New request | `<Name> needs you on <project>` | the request summary | `--acc` |
| Deployed | `<Team> deployed to <project>` | `<N> new sessions starting.` (opens the lead's session) | `--ok` |
| Stopped | `<Team> stopped on <project>` | `Worktrees are kept. Other deployments keep running.` | `--tx4` |
| Project added | `<id> added` | `Wrote .foreman/project.yaml. Deploy a team from its Settings tab.` | `--ok` |
| Config review resolved | `project.yaml` | `Config change merged to main.` (`--ok`) or `Config change discarded.` (`--tx4`) | |
| Board move | `<path>` (e.g. `backlog/LED-44.md`) | `status: <s>`, plus ` · priority: <lane>` when the lane changed | `--info` |
| Notifications enabled | `Notifications on` | `You’ll hear from agents when they finish or need you.` | `--ok` |

## Open questions

1. **Teams index screen.** The prototype has a "Teams" list screen (H1 "Teams", subtitle "Definitions you can deploy to any number of projects. Each deployment runs its own sessions with the same souls.", a `New team` button, rows `<Name> · <Kind> · <N> agents`, `Deployed on ledger, relay` / `Not deployed`, and member avatars) that nothing links to. Options: drop it; make the sidebar "Teams" label a link to it. Recommendation: drop it for v1; the sidebar already lists every team.
2. **Toast click without a session.** Board-move and project-added toasts do nothing on click except dismiss. Options: keep; navigate to the entry or project. Recommendation: navigate to the related screen (the entry in the reader, the project overview) so every toast click goes somewhere.
3. **Responsiveness.** The sidebar never collapses and the board needs 880px of width. PRD asks only for "basic responsiveness". Options: no breakpoints in v1; collapse the sidebar into a menu button under ~760px. Recommendation: one breakpoint that collapses the sidebar; everything else scrolls horizontally.
