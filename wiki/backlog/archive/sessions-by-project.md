---
type: task
status: done
priority: medium
tags: [feature, ui]
epic: terminal-multiplexer
---

# Sessions: a list of deployments, and a page per deployment

**Why:** [Sessions](../../product/features/sessions.md) shows one deployment at a time, picked with tabs at the top, and its agents with a second row of tabs. The CEO wants to see every session at a glance, and to open two sessions in two browser tabs side by side (the CEO, 2026-10-22).

## What the CEO asked

- **A list of sessions, grouped by project**: each project, its deployed teams, their agents' sessions.
- **Each session opens on its own page** (its own URL), so two can be open in two browser tabs.
- **The session selector at the top** (the deployment tabs, and the agent tabs under them) may not be needed any more.
- **The branch the agent is on**, shown with its session.

## Decided (the CEO, 2026-10-25)

- **Each deployment is its own page** (its own URL), replacing the deployment chips at the top, so two deployments can be open in two browser tabs.
- **The agent tabs stay**: the quick way to pick an agent within a deployment.
- **The bar under the tabs is condensed away.** The Copy and Line buttons move to the right end of the agent tabs' row, always visible. The "Direct line to <name>. They'll tell <lead>, the lead, if it matters." banner goes; the agent's **role** shows instead.
- **One branch per agent** in a deployment: `switch(branch)` already moves every code repo of an agent together, and a team can't be deployed twice on one project (`ALREADY_DEPLOYED`). The branch isn't on `Instance` yet: the server has to expose it, live as the agent switches.
- **Activity is a toggle on the deployment's page**, next to the button that opens the project's board. The project's own view may show it later; not now.
- ~~The deployments are listed in the sidebar~~: undone 2026-10-28, see below.
- **Split goes**: two agents side by side are two browser tabs. The URL names the deployment and the agent, no panes.
- **The branch shows on the selected agent's tab** only, so the other tabs stay short.

**Done when:** each deployment has its own page, with the URL as the view; the branch shows; [Sessions](../../product/features/sessions.md) describes the new layout.

## Built (2026-10-25)

The URL stays `/sessions?deployment=&agent=&view=` (`panes` dropped), so every link to a session elsewhere (Dashboard, notifications, Project, Pipeline, Deploy) works unchanged. The page rewrites it to name what it shows, which is what lets the sidebar mark the deployment shown. The branch comes from the project's repos list (`branchOf` over `worktrees`), no server change; it refreshes on `team.action`, and on a status change while the agent shown has none yet (its worktrees are made at its first start).

- [x] `app/sessions.ts`: `activeDeployments` (by project, then team), `sessionsView` (the URL checked, the last view per deployment); the panes, split and drag-to-reorder code removed (`Panes`, `clickTab`, `toggleSplit`, `closePane`, `panesFrom`, `searchFor`, `inOrder`, `moved`); `Segmented` lost its close buttons and drag, used only by the deployment chips
- [x] ~~sidebar: the deployments under Sessions~~ (removed 2026-10-28)
- [x] the page: header (status, team · project, needs-you, Board, Activity, Stop), agent tabs with the shown agent's branch, role · harness, Copy and Line at the row's end; the pane header and the direct-line banner gone
- [x] tests: the deployments' order and dots, the URL (unknown deployment or agent, instance ids, the last view, nothing deployed), `branchOf`, the sidebar's groups
- [x] specs: Sessions, App shell, UI state, Design system, Avatar, the journey, use cases, API, Agent runtime, Project activity

## Changed (the CEO, 2026-10-28)

- **A `/sessions` list page instead of the sidebar rows**: the sidebar has only `Sessions`, active on the list and every deployment's page. The list groups the active deployments by project, each as the Dashboard's At work card (`DeploymentCard`, shared): team, needs-you dot, goal, agents with their status and what they're doing. A card opens today's URL (`/sessions?deployment=…`). Only active deployments (stopped ones live on the Team page).
- `/sessions` with no deployment, or one not deployed any more, is the list (the URL rewritten to `/sessions`), no longer the first deployment.
- Deploy now waits for the new agents to load before opening the page: before, it could open while the instances were stale, and the page fell back to another deployment (now it would fall back to the list).

- [x] `sessionsView` undefined unless the URL names a deployed one; `byProject`; `sessionGroups` carries `needsYou`; the card moved to `components/DeploymentCard.tsx` (with `card`/`cover` in `ui.tsx`); the sidebar rows and `sidebarModel().sessions` removed; `Deploy` awaits the refetch
- [x] tests: the list for no or a stopped deployment, `byProject`, the needs-you flag; the sidebar groups test removed
- [x] specs: Sessions (The list, The URL), App shell, Dashboard, journey, UI state
- [ ] tried by the CEO in the browser (only the unit tests and the build have run it)

## Changed (the CEO, 2026-10-29)

The list "made no sense": what the CEO follows is the project, not which agent is on it, and two teams on one project are rare. So:

- **One full-width row per deployment, the latest activity first**, no project sections. The **project** is the row's name, the **team** a chip on the right. The row also shows the status dot, the needs-you dot, the goal, `working` / `idle` / `asleep` / `stopped`, and the last activity.
- **No agents in the row.** The Dashboard's At work card keeps them: `DeploymentCard` stays on the Dashboard only.
- **Last activity**, shown relative (`5m ago`), is the latest agent step (`doing.at`). Steps are kept in memory, so after a restart the latest agent `startedAt` stands in, then the deployment's `createdAt`.

- [x] `sessionRows` in `app/sessions.ts` (built on `activeDeployments`), `SessionsRow` in `screens/Sessions.tsx`; `byProject` removed (it had no other user)
- [x] tests: the order by activity (ties included), the fallbacks with Go's zero time ignored, a step beating a later start, each state word
- [x] the last activity relative (`ago` in `app/model.ts`, beside `when`), the list redrawn every 30 s; tests: each unit boundary, a time ahead of the clock
- [x] specs: Sessions (The list), Dashboard (no longer shared)
- [ ] tried by the CEO in the browser

## Changed (the CEO, 2026-10-30)

- **Sorted on entering only.** A list that reorders while the CEO is on it makes no sense: the latest activity first when the page opens, then the order holds; a deployment new since goes on top, one gone drops out (`keepOrder` in `app/sessions.ts`).
- **One grouped panel**, as on Reviews: corners rounded only at its top and bottom, a line between rows.
- **A second line under the project**: the team's chip, then the start of the goal.

- [x] `keepOrder`, the list in a `Panel`; tests: the entering order, held against new activity, a new deployment on top, one gone dropped
- [x] spec: Sessions (The list)
- [ ] tried by the CEO in the browser

## Changed (the CEO, 2026-10-30): Sessions as the router for a project

- **The header's buttons**: `Team` (this page, shown on), `Reviews` (the Reviews page narrowed to the project), `Board`, and a `Deployment ▾` menu (Show activity, Project settings, Stop deployment). Links to the existing pages, not tabs inside the session ("tabs within tabs?"); the project page's `<Team> · Sessions` button is the way back.
- **Reviews takes `?project=`**, with a Project select on the page (the CEO's idea: the session's link applies the filter).
- **`⌘⇧←` / `⌘⇧→`** (Ctrl+Shift off a Mac) move to the previous / next agent, round the ends.

- [x] `DeploymentMenu` (Radix dropdown), the `/reviews` route's `project`, `reviewProjects`, `agentStep` / `stepAgent`; tests for the helpers and the keys (each platform, the keys left to the agent, the ends)
- [x] specs: Sessions (Header, Agent tabs), Reviews
- [ ] tried by the CEO in the browser

## Superseded (the CEO, 2026-10-31)

The Sessions list and its `/sessions` URL went with [The project is the workplace](../project-workplace.md): a session is now the Session section of its project. The unticked "tried by the CEO" steps above carry over to that card.
