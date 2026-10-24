---
type: task
status: in-progress
priority: medium
tags: [feature, ui]
epic: terminal-multiplexer
---

# Sessions: a page per deployment, listed in the sidebar

**Why:** [Sessions](../product/features/sessions.md) shows one deployment at a time, picked with tabs at the top, and its agents with a second row of tabs. The CEO wants to see every session at a glance, and to open two sessions in two browser tabs side by side (the CEO, 2026-10-22).

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
- **The deployments are listed in the sidebar**, under Sessions, grouped by project, each with its status and needs-you dot. No `/sessions` list page.
- **Split goes**: two agents side by side are two browser tabs. The URL names the deployment and the agent, no panes.
- **The branch shows on the selected agent's tab** only, so the other tabs stay short.

**Done when:** each deployment has its own page, with the URL as the view; the branch shows; [Sessions](../product/features/sessions.md) describes the new layout.

## Built (2026-10-25)

The URL stays `/sessions?deployment=&agent=&view=` (`panes` dropped), so every link to a session elsewhere (Dashboard, notifications, Project, Pipeline, Deploy) works unchanged. The page rewrites it to name what it shows, which is what lets the sidebar mark the deployment shown. The branch comes from the project's repos list (`branchOf` over `worktrees`), no server change; it refreshes on `team.action`, and on a status change while the agent shown has none yet (its worktrees are made at its first start).

- [x] `app/sessions.ts`: `activeDeployments` (by project, then team), `sessionsView` (the URL checked, the last view per deployment); the panes, split and drag-to-reorder code removed (`Panes`, `clickTab`, `toggleSplit`, `closePane`, `panesFrom`, `searchFor`, `inOrder`, `moved`); `Segmented` lost its close buttons and drag, used only by the deployment chips
- [x] sidebar: the deployments under Sessions, grouped by project (`sidebarModel().sessions`)
- [x] the page: header (status, team · project, needs-you, Board, Activity, Stop), agent tabs with the shown agent's branch, role · harness, Copy and Line at the row's end; the pane header and the direct-line banner gone
- [x] tests: the deployments' order and dots, the URL (unknown deployment or agent, instance ids, the last view, nothing deployed), `branchOf`, the sidebar's groups
- [x] specs: Sessions, App shell, UI state, Design system, Avatar, the journey, use cases, API, Agent runtime, Project activity
- [ ] tried by the CEO in the browser (only the unit tests and the build have run it)
