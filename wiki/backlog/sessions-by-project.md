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

## Proposed shape

- `/sessions`: the list, by project, then team, then agent: status, branch, what it is doing, the needs-you dot.
- `/sessions/<agent@project>`: one terminal, under a header (agent, team, branch, status, back to the list), with a teammates menu to jump within the team, and an Activity toggle for its team.
- **Split goes**, and the agent tabs with it: two browser windows side by side do it, tiled by the OS, and the teammates menu replaces the tabs. The URL loses its panes. Two tabs on one session already work (the most recently focused one sets the terminal's size).

## Open questions

- **Split and the agent tabs going**, as proposed?
- **The branch:** per repo (an agent's worktrees can be on different branches, though `switch` keeps them together), live as it switches (a `team.action` already says so), and where: the pane header, the list row, or both?
- **Activity**, today beside a terminal in the Sessions view: on the session's page, the project's Activity tab, or both?

**Done when:** the questions are answered; the list and the session page are built, with the URL as the view; the branch shows; [Sessions](../product/features/sessions.md) describes the new layout.
