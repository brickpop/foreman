---
type: spec
status: built
tags: [feature, ui, pipeline, requests]
---

# Reviews

Where the CEO reviews a lead's branch as on a forge, and decides it (the CEO, 2026-10-21): the files it changed, their diff, comments pinned to lines or general, sent back to the lead as a request for changes, and the merge. Reviews are listed and decided **only here** (the CEO, 2026-10-21: no review cards on the [Dashboard](./dashboard.md) or a project's [Needs you](./project-pipeline.md#1-needs-you)); a repo's **Review** button in the project's [In flight](./project-pipeline.md#2-in-flight) opens its page. The model is [Review](../models/review.md); the flow around it, [Pipeline and merges](./pipeline-and-merges.md#review-and-merge).

## The Reviews page

`/reviews`, the sidebar's **Reviews** entry, across projects (or one: `?project=<id>`, chosen with a `Project` select at the title's right, `All projects` or each project with a review, and the one asked for; a session's **Reviews** links it, the CEO, 2026-10-30), with the amber count of reviews that need the CEO ([App shell](./app-shell.md#sidebar)). Same width and title as the Dashboard: `Reviews`, subtitle `The leads' branches on their way to main.`

Three sections, each left out when empty, one panel of rows each: **Needs you** (`needs-review`), **Waiting on the lead** (`changes-requested`) and **Done** (merged, dismissed or withdrawn, the last 20). The server lists them newest first (the latest opened).

- **Row** (grid `auto 1fr auto`, padding `12px 16px`, the whole row a link to its page): Avatar 26 of the lead; the entry's title (500), then `<project> · <Lead> · <repos> · <branch>` (branch in Mono); on the right the status (`Needs your review`, `Changes requested`, `Merged`, `Dismissed`, `Withdrawn`) over its time.
- **Empty**: `No reviews yet. A lead's request_merge() opens one.`; narrowed to a project, `No reviews on <project> yet. Its lead's request_merge() opens one.`

## A review's page

`/reviews/<id>`, with `?since=1` for the change since the last review.

- **Header**: a square `←` button (32px, outline; back to Reviews) left of the entry's title (26px), then the lead's avatar and `<Lead> on <project> · <branch> · <status> · <time>` (the project links to its Worktrees tab, the branch to its code in Repositories, in the first repo of the review). A withdrawn review adds `Withdrawn: <reason>.`, then `Superseded by <review>.` (a link to it) when one holds all of it.
- **Which change** (a segmented control, only once the CEO requested changes on a repo still open): **Against main** (the default: what a merge takes, `main...branch`) or **Since your last review** (from the branch head at the last Request changes to its head now).
- **Layout**: the header on top; under it two panes, each scrolling on its own (the tree, and the diffs with the comments), so going to a file moves only the diffs and the tree stays where it was (the CEO, 2026-10-21); the decision bar at the foot.
- **File tree** (240px, on the left, a `--line` border to its right; hidden under 900px): one group per repo open in the review, `<repo> → <main>`, then its files (rows padded `6px 8px`) with `+added −removed`; a click unfolds the file and scrolls the diffs to it. `No changes.` for a repo with none.
- **Diffs**: one box per file, 20px apart, bordered `--line2`. Its header (`--raised`, the `▾`/`▸` fold mark, `<repo> <path>`, `new`, `deleted` or `renamed`, and `+added −removed`) stays at the very top of the pane while its lines scroll, covering what's above it (square once stuck: the box's rounding is clipped). A click on it folds the file to just the header, or unfolds it; folding a file whose header is stuck scrolls back to the file's start. Each line has its number on `main`'s side and on the branch's, then its text, colored added, removed or kept; hunk headers in `--tx4`.
- **Line comments**: hovering a line shows a `+` in its gutter, while the review is open. It opens a box under the line (`What should change here?`, `Cancel`, `Add to the draft`). A kept or added line is commented on the branch's side, a removed one on `main`'s: only against main, since the other view's old side is the branch as it was. The line's comments sit under it, at the line they moved to.
- **General comments**, after the diffs: the general ones, and a box `A comment on the whole change…`. Then, listed apart with their `<repo> <path>:<line>` and the line's text: **Outdated** (their line changed since) and **Not in this view** (their repo was merged or dismissed, or the line isn't in the change shown).
- **A comment** shows `draft` (amber outline) with **Edit** and **Delete**, or `sent <time>`. A sent one can't change.
- **Decision bar** (at the foot, `--side`, while the review is open): when changes are requested, `Changes requested: waiting on <Lead> to fix them and ask again.`; a checkbox per repo still open, all ticked at first, labeled with its link line (`<repo> · <branch> → <main> · checks <…>`, or `· conflicts with <main>` in `--err`); then **Dismiss** (danger), **Request changes (n)** (the draft count) and **Merge to <main>** (primary; `Merge` when the ticked repos' mains differ; disabled with none ticked, or until the ticked repos' diffs have loaded).
  - **Request changes** opens a note (`A note for <Lead>, with the n draft comments (optional)`) and `Send to <Lead>`, which needs a draft or a note.
  - **Dismiss** (the work isn't wanted) opens **What becomes of it**, radios, the first one picked. While the lead's deployment runs: `<Team> keeps working on <project>; the branches stay`, `Stop <Team> on <project>; the branches stay`, `Stop <Team> on <project> and drop the branches (kept aside, recoverable by hand)`. The branches go only with the team, as its agents work in them. Once it no longer runs: `Keep the branches`, `Drop the branches (kept aside, recoverable by hand)`. Only when the team keeps working is there a note (`A note for <Lead> (optional): <Lead> is told to stop work on it either way`); a note typed, then a choice that won't send it, stays in view with a warning in `--err`: `<Lead> won't get your note: the team stops.` (or `… the team isn't running.`). The button reads `Dismiss now`, or `Dismiss and send` with a note (the CEO, 2026-10-21). A branch that couldn't be dropped (a git error) shows in a toast, `Dismissed, but some branches stay`, a line each: `<branch> in <repo>: <why>.`
  - **Merge** merges each ticked repo; a toast says which merged. One that conflicts stays open, marked, and the toast names it with the reason.
  - **Only what the CEO read** (the CEO, 2026-10-21): Merge sends, for each ticked repo, the head its diff was shown at, and a repo whose branch moved since (the lead committed meanwhile) isn't merged: the toast says `<repo>: the branch has new commits since you loaded this page. Read them, then merge again`, and the page reloads its diff. The other repos merge as usual.

## What the lead gets

**Request changes** sends one message to the lead, typed into its session when idle:

```
From the CEO: changes requested on lead/atlas/invoices (backlog/invoices.md).

<the note>

- api internal/calc.go:5 `total := a + b`: Round it to cents.

- Split the migration out.

Fix them, commit(), then call request_merge again on the same branch.
```

The comments come in the order they were written: a line comment as `<repo> <path>:<line>` (its line now; `<repo> <path>, removed line <n>` on `main`'s side; ` (changed since)` once outdated) with the line's text, a general one as its text. The review request resolves, and the review waits on the lead until its next `request_merge` on the branch, which raises it again with the new diff.

**Dismiss** means the work isn't wanted: it ends the review unmerged in every repo still open. While the team keeps working, the lead is told to stop, by Foreman, note or not: `From Foreman: the CEO dismissed the review of <branch> without merging (<repos>). Stop work on it, and don't request it again.`, then ` The CEO's note: <note>` when there is one. A team being stopped, or no longer running, is told nothing: there is nobody to read it. Its work items stay with the dismissed merge requests, so they leave In flight ([Pipeline and merges](./pipeline-and-merges.md#review-and-merge)).

## Withdrawn

A review can end before the CEO decides it, **withdrawn** (the CEO, 2026-10-22): its work went into another of the lead's branches, or the lead dropped it.

- **By the lead**, with `withdraw_merge(branch, reason, superseded_by?)`, on its own open review only, all of it ([Protocol → `withdraw_merge`](../architecture/protocol.md#withdraw_merge)). `superseded_by` names another of its branches with a review open that holds all of this one.
- **By Foreman**, when the CEO merges a review: every other open review of the same lead that `main` then holds all of is withdrawn, superseded by the merged one, and its lead told: `From Foreman: the CEO merged <big>, which holds all of <small>, so Foreman withdrew the review of <small> (<id>, <repos>). Don't request it again.` Nothing is withdrawn when a lead requests a bigger branch: both stay open, and the CEO can merge the smaller one and dismiss the bigger (merging the smaller leaves the bigger open: it holds more).
- Either way the review leaves Needs you (its request resolves), shows **Withdrawn** under Done, and the CEO's **draft comments on it are deleted**: they can't be sent any more. A toast says so: `<Lead> withdrew <branch>` (or `Foreman withdrew …`) / `<reason>.`, then ` Your <n> draft comments on it are lost.` (in `--err`) when there were any. The branch stays. Its work items are freed (their commits are still on the lead branch): they go with the superseding review, or the lead's next request of a branch that holds them, unlike Dismiss, after which they stay with the dismissed merge requests.

## Phasing

Built (2026-10-21, [the review page card](../../backlog/review-page.md)). Withdrawn (2026-10-22, [its card](../../backlog/archive/withdraw-merge-request.md)). Not yet: comments on a range of lines, threads (answers from the lead), **Create PR** on the forge from the decision bar ([The forge](../../backlog/git-forge.md)). A merge request from before reviews that was already dismissed has no page. Repos already merged or dismissed show no diff on an open review.
