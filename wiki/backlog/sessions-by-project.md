---
type: task
status: todo
priority: medium
tags: [feature, ui, decision]
epic: terminal-multiplexer
---

# Sessions: a list by project, each session on its own page

**Why:** [Sessions](../product/features/sessions.md) shows one deployment at a time, picked with tabs at the top, and its agents with a second row of tabs. The CEO wants to see every session at a glance, and to open two sessions in two browser tabs side by side (the CEO, 2026-10-22).

## What the CEO asked

- **A list of sessions, grouped by project**: each project, its deployed teams, their agents' sessions.
- **Each session opens on its own page** (its own URL), so two can be open in two browser tabs.
- **The session selector at the top** (the deployment tabs, and the agent tabs under them) may not be needed any more.
- **The branch the agent is on**, shown with its session.

## Open questions

- **The selector:** the list replaces the deployment tabs. The agent tabs are still the quick way to jump to a teammate, and to pick the second pane of Split: keep them on a session's page, go back to the list for that, or keep Split at all, now that two browser tabs do the same?
- **The branch:** per repo (an agent's worktrees can be on different branches, though `switch` keeps them together), live as it switches (a `team.action` already says so), and where: the pane header, the list row, or both?
- **Activity**, today beside a terminal in the Sessions view: on the session's page, the project's Activity tab, or both?

**Done when:** the questions are answered; the list and the session page are built, with the URL as the view; the branch shows; [Sessions](../product/features/sessions.md) describes the new layout.
