---
type: task
status: done
priority: medium
tags: [feature, ui]
epic: workflow
---

# More touches: Session dots, Inbox, Wiki layout, Deployment split, Board accents, Settings grouping

**Why:** the CEO's feedback after merge targets (2026-11-02): several pages work but read as unfinished or inconsistent with their neighbours.

## Steps

- [x] **Session sidebar item**: a status dot on each agent's item when it isn't idle (the same colors as elsewhere for working, waiting on the CEO, stopped). Spec: [Sessions](../../product/features/sessions.md), [App shell](../../product/features/app-shell.md#sidebar).
- [x] **Inbox**: section titles that read as such (`Needs you`, `Waiting on the lead`, `Done`), and a calm placeholder when nothing is open (`Nothing needs you yet.`). Spec: [Reviews](../../product/features/reviews.md), [Requests](../../product/features/requests.md).
- [x] **Wiki**: a header with a title (the project's knowledge base), the host path under it as dim text, then a file tree that looks like one (folder and file icons, indentation guides). Read pages styled as a repo's file page is in Project → Repos → a repo (the same header band, file path, body box), so the two readers look alike. Spec: [Wiki reader](../../product/features/wiki-reader.md), [Repos → A repo's page](../../product/features/repos.md#a-repos-page).
- [x] **Deployment**: the agents of the team deployed now apart from those of earlier teams (their worktrees still on disk); for those, an action to clean what they left (worktrees, and their `wt/*` branches already in a target), confirmed first. Spec: [Project deployment](../../product/features/project-team.md); it overlaps [Remove worktrees](./remove-worktrees.md).
- [x] **Deployment layout**: the selector (decided below).
- [x] **Board**: the selector of the available boards gets an accent color on the one chosen; the same on Deployment's selector, once it has one. Spec: [Board](../../product/features/board.md).
- [x] **Board → wiki.toml**: shown in a popover that closes on a click outside, instead of expanding in the page.
- [x] **Global settings** grouped like Project settings (titled cards of related rows), keeping its tabs. Spec: [Settings](../../product/features/settings.md), [Project settings](../../product/features/project-settings.md).
- [x] **Sidebar: the projects only** (2026-11-02): the teams moved to Settings → Teams, the first tab, the Dashboard's cards (one `TeamGrid`); team and agent pages at `/settings/teams/…` in the Settings frame (760px) with a breadcrumb; every Settings tab a path (`/settings/<tab>`); old `/teams/…` and `?tab=` links redirect. Spec: [Settings → Teams](../../product/features/settings.md#teams), [App shell](../../product/features/app-shell.md#sidebar).
- [x] Specs updated; tests for the logic (the Session dot, the Deployment view, the cleanup rules, git's worktree removal)

## Decided (the CEO, 2026-11-02)

- **Deployment layout**: a selector at the top, `Agents · In flight · Merged`, one shown at a time (the Board's selector style), opening on Agents while a team runs.
- **Cleaning an earlier team's agents** removes only what is safe: worktrees with nothing uncommitted, and `wt/*` branches a target already holds; anything else is listed and kept, as Dismiss does. Confirmed first.
