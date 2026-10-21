---
type: task
status: done
priority: medium
tags: [feature, decision]
epic: workflow
---

# The lead withdraws a request to merge

**Why:** a review can stop making sense before the CEO decides it: another lead branch supersedes it (its work went into a bigger branch), or the lead dropped the approach. Only the CEO could end it (Close, in [Reviews](../../product/features/reviews.md)), so it stayed in "Needs you" until they read it and closed it by hand, and an agent asked for a tool it didn't have (the CEO, 2026-10-21).

## Decisions (the CEO, 2026-10-22)

- **The whole review**, never some repos: a superseding branch replaces all of it.
- **A status of its own, Withdrawn**: "the lead gave up on it" and "the CEO turned it down" read differently in history.
- **The CEO's draft comments are lost**, and the CEO is warned (a toast with the count).
- **Foreman withdraws on a merge, never at request time**: when the CEO merges a review, the lead's other open reviews that `main` then holds all of are withdrawn, superseded by it. Both stay open until then, so the CEO can merge the smaller one and close the bigger.

## Built

- `withdraw_merge(branch, reason, superseded_by?)`, the lead's ([Protocol → `withdraw_merge`](../../product/architecture/protocol.md#withdraw_merge)); `superseded_by` must hold all of the branch (git ancestry) in each repo open.
- Merge request state `withdrawn`; review `withdrawn_at`, `withdrawn_reason`, `superseded_by` (migration 006); review status `withdrawn` ([Review](../../product/models/review.md)).
- Work items freed, their commits still on the lead branch: to the superseding merged merge request, else the lead's open ones that hold them, else free for the next `request_merge`.
- Foreman's withdrawal in `merged()`, before the "main moved" notices; `Reconcile` re-reads each merge request, so one withdrawn by an earlier one's merge isn't then taken for merged.
- `review.withdrawn` event: the toast, Reviews refreshed; the review page shows the reason and links the superseding review.
- Tests: `internal/workflow/withdraw_test.go` (refusals, the lead's withdrawal with hand-offs moved and drafts lost, freed hand-offs taken by the next request, a merge withdrawing only once `main` holds every repo, merging the smaller keeping the bigger, a merge outside Foreman); store, prompt, UI notification and status tests.

## Left open

- Nothing on the Merge button says another review will be withdrawn; the toast says it after.
- A merge noticed outside Foreman (`Reconcile`, oldest merge request first): a held review older than the one merged is found merged itself before the merged one is reached, so it shows Merged instead of Withdrawn. Harmless (nothing is lost, nobody is told to merge `main` into it), but not superseded.
