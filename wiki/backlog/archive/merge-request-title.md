---
type: task
status: done
priority: medium
tags: [feature]
epic: workflow
---

# A merge request's title and description

**Why:** the CEO, 2026-11-04: merge requests "should also accept the title... WITH an optional description (expandable on the PR view)". Decided: "Linking to the board card is nice, but the title should be explicitly set by the agent, and the optional description expandable". The card stays required, and is linked.

**Built (2026-11-04):**

- [x] `request_merge(branch, title, body?, entry, into?)`: a title of one line and at most 120 characters (required; `teamtools.BadTitle`, shared with `escalate` and `contribute`), a body of at most 8,000; kept on the [review](../../product/models/review.md) (migration `019_review_title.sql`), a refresh replacing both
- [x] the review is named by its title everywhere: the Inbox, its page, the pipeline's rows (`useReviewTitle`), the lead's `reviews()`, its request's summary (`Merge <branch>: <title>`); the API's `entryTitle` is the card's
- [x] its page: `· card <card title>` (a link to the wiki entry) after the status, then the body, folded past 6 lines or 480 characters (`FoldedText`, shared with the request panel)
- [x] the prompt's tool line and the branches sentence
- [x] tests: the refusals (no title, two lines, too long, body too long), a refresh's title and body replacing the old, a body left out leaving none (`TestRequestMergeRefusals`, `TestFromHandoffToMain`); `FoldedText.test.tsx`
