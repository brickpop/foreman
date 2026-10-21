---
type: task
status: todo
priority: high
tags: [feature]
epic: workflow
---

# The lead reads its reviews: `reviews()`

**Why:** the lead hears about its reviews only through notices typed into its session (merged, changes requested with the comments, dismissed, withdrawn, conflicts, main moved). Once its context is compacted or it restarts, it can't tell which reviews are open, which wait on it, or what the CEO's comments said (the CEO, 2026-10-21: "yes" to a read-only tool).

## What it does

- `reviews()`, lead only, read-only: the lead's reviews on the project, open ones and the ones decided in the last 7 days, newest first.
- Each one: its id, branch, entry and title; per repo its state (waiting for the CEO · changes requested · conflicts with main · merged · dismissed · withdrawn) and, when dismissed, the CEO's note if any.
- While changes are requested: the CEO's note and the comments sent, each on its current line (`api/cal.go:42`) or marked outdated, as the [review page](../product/features/reviews.md) places them, general comments last.
- The text is plain, like the other tools' results; nothing is written.

## Built from

What `Review` and its comment placement already compute for the page ([Review](../product/models/review.md), `workflow.Review`), so the tool and the page can't disagree.

**Done when:** the tool is in [Team tools](../product/architecture/team-tools.md) and [Protocol](../product/architecture/protocol.md), offered to the lead only, and tested: a lead sees its own reviews only (not another team's on the same project), each state, comments placed and outdated, nothing older than 7 days once decided.
