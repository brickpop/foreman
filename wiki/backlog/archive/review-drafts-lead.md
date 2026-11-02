---
type: task
status: done
priority: high
tags: [feature, ui]
epic: workflow
---

# Review: drafts lead the decision, Request changes opens the lead's session

**Why:** the CEO's feedback on a review (2026-11-03): staged comments should make sending them the obvious next step, not Merge; after sending, the CEO wants to watch the lead pick them up; `Add to the draft` reads as secondary; a checkbox on a single repo means nothing.

**Decided (the CEO, 2026-11-03),** written into [Reviews](../../product/features/reviews.md):
- `Add to the draft` is primary.
- With one or more draft comments, the bar reads Dismiss, Merge (default style), **Request changes (n)** (primary, last). Merge asks first: `<n> draft comments won't be sent and will be lost. Merge anyway?`; merged, the drafts are deleted. With none, as today.
- Request changes, sent (comments or just a note), opens the lead's session instead of the Inbox. Merge and Dismiss still go back to the Inbox.
- The per-repo checkbox only when more than one repo is open (a review spans one merge request per repo, merged together or apart).

- [x] the button styles and order, from the draft count
- [x] the confirmation before Merge with drafts
- [x] the server deletes a review's drafts when its last repo merges (`MergeReview`, through the withdrawal's `dropDrafts`; until then they stayed on a closed review, unseen)
- [x] Request changes navigates to `sessionOf(lead)`
- [x] no checkbox with one open repo; Merge still sends that repo's head
- [x] tests: `TestMergingTheLastRepoDropsTheDrafts` (kept while a repo is open, gone after the last), `lostDrafts` (`reviews.test.ts`). The bar's order, the confirmation and the navigation have no component test yet ([UI component tests](../ui-component-tests.md))
- [x] [Reviews](../../product/features/reviews.md) as built (drop the "planned" notes)
