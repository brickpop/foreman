---
type: spec
status: built
tags: [model, pipeline, git, requests]
---

# Review

The CEO's review of one lead branch, stored in `state.db`: what the CEO reads and decides on the [Reviews](../features/reviews.md) page. It groups the [merge requests](./merge-request.md) a lead's `request_merge` opened on that branch, one per repo the branch changed, so a branch is reviewed once whatever the repos (the CEO, 2026-10-21). It carries the CEO's **comments** to the lead, and one [review request](./request.md) while it waits on the CEO.

## Schema

Table `reviews`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique, `rev-<n>` | |
| `project_id` | String | yes | | | The project |
| `branch` | String | yes | | | The lead branch `lead/<team>/<topic>` |
| `created_at` | Timestamp | yes | now | | |
| `changes_requested_at` | Timestamp | | | | Set by Request changes; cleared by the lead's next `request_merge` |

Table `review_comments`.

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `id` | String | yes | | unique, `com-<n>` | |
| `review_id` | String → Review | yes | | | |
| `repo` | String | | | a repo open in the review | Empty for a general comment |
| `path` | String | | | | The file, for a line comment |
| `side` | String | | | enum: `new`, `old` | `new`: a line of the branch (kept or added); `old`: a line `main` had and the branch removed |
| `line` | Integer | | | > 0 | Its number on that side, when written |
| `commit_sha` | String | | | a commit hash | The branch head it was read at |
| `snippet` | String | | | ≤ 500 characters | The line's text, quoted in the message to the lead |
| `body` | String | yes | | ≤ the team tools' message limit | |
| `state` | String | yes | `draft` | enum: `draft`, `sent` | |
| `created_at` | Timestamp | yes | now | | |
| `sent_at` | Timestamp | | | | Set by Request changes |

On `merge_requests`: `review_id` (the review it is decided in) and `reviewed_head` (the branch head at the CEO's last Request changes, the start of "since your last review"). On `requests`: `review_id`, for a `review` request.

## Status

Derived, never stored:

| Status | When |
|---|---|
| `needs-review` | a merge request is open and changes aren't requested |
| `changes-requested` | a merge request is open and the CEO requested changes: it waits on the lead |
| `merged` | nothing open, and something merged |
| `closed` | nothing open, nothing merged |

## Invariants

- **A branch's open review** is the review of any of its open merge requests. `request_merge` adds the repos it opens to it; with none open, it starts a new review. So a branch merged (or closed) and requested again gets a new review, and the old one stays as it was.
- **One review request per review**, raised only while it needs the CEO: by `request_merge` when the review has an open merge request, no open request, and changes aren't requested. Request changes resolves it (`changes requested`); the lead's next `request_merge` clears `changes_requested_at` and raises a new one. Once no merge request is left open it resolves (`merged` or `closed`).
- **Comments are drafts until Request changes**, which sends every draft, and the CEO's optional note, to the lead in one message, then marks them `sent`. Only a draft can be edited or deleted; comments are written only while the review is open, and a line comment only on a repo open in it.
- **A sent comment follows its line.** One on the `new` side moves as the lead's commits shift it (`git diff -U0 <its commit> <head> -- <path>`), and is **outdated** once its own line changed. One on the `old` side is outdated once its file changed. One whose commit is gone (history rewritten) is outdated. A comment in a repo no longer open stays where it was written.
- **Merging is per repo:** Merge takes the repos the CEO ticked; each merges [as before](../features/pipeline-and-merges.md#merge-to-main), and those left out stay open on the review.
- Merge requests opened before reviews existed: open ones were grouped into one review per (project, branch); closed ones have none.

## Phasing

Built (2026-10-21, [the review page card](../../backlog/review-page.md)): the tables, the grouping and the review requests, comments with line following, Request changes, Merge (repos) and Close. Not yet: comments on a range of lines, threads (the lead answering a comment), Create PR from a review ([The forge](../../backlog/git-forge.md)).
