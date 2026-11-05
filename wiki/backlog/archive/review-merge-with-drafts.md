---
type: task
status: done
priority: high
tags: [feature]
epic: workflow
---

# Don't let Merge swallow a review in progress

**Why:** the CEO, 2026-10-26. While writing comments on a review, they clicked **Merge to main** instead of sending the comments, out of habit. A merge into `main` can't be undone from Foreman. What the code does today (`ui/src/screens/Review.tsx`, the decision bar; `internal/workflow/merges.go`):

- **Merge** is the primary button, right next to **Request changes (n)**, and merges on one click, with no confirmation, even while the review has draft comments.
- **The drafts are then left behind.** Merging doesn't touch the comments: they stay as drafts on a merged review, can't be sent any more, and nothing tells the CEO. Withdrawing a review at least deletes them and says so ([Reviews → Withdrawn](../../product/features/reviews.md)); a merge doesn't even do that.
- The two actions look alike (buttons next to each other), but writing comments means "not ready yet", and merging means "ready". The page doesn't tie the decision to what the CEO has been writing.

**Done when:** a merge can't happen by accident while the CEO has unsent comments, the CEO has agreed to the new decision bar, and [Reviews](../../product/features/reviews.md) describes it.

**To decide with the CEO before building** (options, from the least to the most change):
- **A warning.** With drafts, **Merge** asks first: `You have n unsent comments. Merge anyway? They will be discarded.` with **Send them instead** as the default. Simplest; the CEO asked for at least this.
- **Merge disabled while drafts exist.** It says why (`n draft comments: send or delete them first`). No dialog to click through by reflex.
- **The primary action follows the draft.** With no drafts, **Merge** is the primary button; as soon as there is one, **Request changes (n)** becomes primary and **Merge** drops to a secondary or a menu. The page's main button always matches what the CEO is doing.
- **One "Finish review" step, as forges do.** A single **Finish review** button opens a choice: Request changes (with the drafts and a note), Merge, or Dismiss, with Request changes preselected when there are drafts. Merging is always a deliberate second step.
- In every option: **what happens to drafts on a merge.** Discard them (with a toast, as on a withdrawal), or send them to the lead as notes after the merge (follow-ups, nothing blocking).

**Also check:** Dismiss with drafts (the same question: it already opens a form first, but what becomes of the drafts is to be checked), and a review whose drafts are only on a repo the CEO unticked.

**Built (2026-11-03):** the primary action follows the draft, and Merge with drafts asks first; merged in every repo, the drafts are deleted ([Reviews → Decision bar](../../product/features/reviews.md#a-reviews-page), `TestMergingTheLastRepoDropsTheDrafts`). Dismiss with drafts opens its form first, as before.

- [x] agree on the option with the CEO
- [x] the decision bar, and the server deciding what becomes of drafts on a merge (not left as orphans)
- [x] tests
- [x] update [Reviews](../../product/features/reviews.md)
