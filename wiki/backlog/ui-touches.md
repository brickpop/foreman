---
type: task
status: todo
priority: medium
tags: [feature, ui]
epic: workflow
---

# More touches: Session dots, Inbox, Wiki layout, Deployment split, Board accents, Settings grouping

**Why:** the CEO's feedback after merge targets (2026-11-02): several pages work but read as unfinished or inconsistent with their neighbours.

## Steps

- [ ] **Session sidebar item**: a status dot on each agent's item when it isn't idle (the same colors as elsewhere for working, waiting on the CEO, stopped). Spec: [Sessions](../product/features/sessions.md), [App shell](../product/features/app-shell.md#sidebar).
- [ ] **Inbox**: section titles that read as such (`Needs you`, `Waiting on the lead`, `Done`), and a calm placeholder when nothing is open (`Nothing needs you yet.`). Spec: [Reviews](../product/features/reviews.md), [Requests](../product/features/requests.md).
- [ ] **Wiki**: a header with a title (the project's knowledge base), the host path under it as dim text, then a file tree that looks like one (folder and file icons, indentation guides). Read pages styled as a repo's file page is in Project → Repos → a repo (the same header band, file path, body box), so the two readers look alike. Spec: [Wiki reader](../product/features/wiki-reader.md), [Repos → A repo's page](../product/features/repos.md#a-repos-page).
- [ ] **Deployment**: the agents of the team deployed now apart from those of earlier teams (their worktrees still on disk); for those, an action to clean what they left (worktrees, and their `wt/*` branches already in a target), confirmed first. Spec: [Project deployment](../product/features/project-deployment.md); it overlaps [Remove worktrees](./remove-worktrees.md).
- [ ] **Deployment layout**: to decide (below).
- [ ] **Board**: the selector of the available boards gets an accent color on the one chosen; the same on Deployment's selector, once it has one. Spec: [Board](../product/features/board.md).
- [ ] **Board → wiki.toml**: shown in a popover that closes on a click outside, instead of expanding in the page.
- [ ] **Global settings** grouped like Project settings (titled cards of related rows), keeping its tabs. Spec: [Settings](../product/features/settings.md), [Project settings](../product/features/project-settings.md).
- [ ] Specs updated; tests for the logic (the agent split, the cleanup refusals)

## To decide with the CEO

- **Deployment layout.** Today: the agents, then In flight and Merged side by side. Options: (a) In flight first, since it is what moves; (b) a selector at the top, `Agents · In flight · Merged`, one shown at a time (the Board's selector style). Recommendation: (b), defaulting to Agents while a team runs, since each list then gets the full width and the page stops growing.
- **Cleaning an earlier team's agents**: what goes. Recommendation: worktrees with nothing uncommitted and branches a target already holds; anything else is listed and kept, as Dismiss does.
