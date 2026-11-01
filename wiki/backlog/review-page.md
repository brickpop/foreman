---
type: task
status: in-progress
priority: high
tags: [feature]
epic: workflow
---

# A review page: the change, file by file, with comments to the lead

**Why:** a review request today is decided on its card (Merge to main · Create PR · Reject), with the change one click away in a dialog. The CEO wants to review as on a forge before deciding (the CEO, 2026-10-21): the files changed, the diff, and comments pinned to lines or general, sent back to the lead as a request for changes. The decision belongs on that page, not on the card.

Since 2026-10-31 reviews are listed in each project's Inbox, not in a Reviews section across projects ([The project is the workplace](./project-workplace.md)); the decisions below are as made on 2026-10-21.

## Asked (the CEO, 2026-10-21)

- A **Reviews** section: one entry per review, and the review card (Dashboard, Needs you) links to it instead of deciding.
- On the review: a **tree of the changed files**, each click scrolling to its diff; the **diff**, per file ([the diff view](../product/features/project-pipeline.md) as it is now: one box per file); **comments**, on a line or a range of the diff, or general, sent to the lead as a request for changes.
- The **decision is made only there**: Merge to main, Request changes. Create PR on the forge later ([The forge](./git-forge.md)).
- What happens after a merge: [After a merge: clean up](./after-merge-cleanup.md).

## Decided (the CEO, 2026-10-21)

- **One review per lead branch**, covering every repo it changed: the file tree is grouped by repo. Each repo is still its own merge request: the CEO can merge some and not others.
- **Comments are a draft** until **Request changes** sends them to the lead in one message, each with its repo, file and line (or general).
- **Reviews is a sidebar section, across projects**, like an inbox; the review card on the Dashboard and in a project's Needs you opens its review there.
- **No threads yet:** the lead acts on the comments and calls `request_merge` again; the review then shows the new diff, and a comment whose line has changed since is marked outdated. Answers in a thread (a team tool) come later.
- **One Merge to main** for the review, every repo ticked by default; the CEO unticks any to leave it out, and a repo left out stays open on the review.
- **The diff is against main** (what a merge takes), with a toggle for only what changed since the CEO last requested changes.

## Plan (2026-10-21)

- **Model:** a `reviews` table (`rev-N`: project, branch, created, `changes_requested_at`) groups the merge requests one `request_merge` opens on a branch, one per repo as now (`merge_requests.review_id`, and `reviewed_head`: the branch head at the CEO's last Request changes, for "since your last look"). A branch's open review is the one any of its open merge requests carries; none open, the next `request_merge` starts a new one. Its status is derived: needs review, changes requested (waiting on the lead), merged (some merged, none open), closed. Merge requests from before keep no review: only open ones are grouped.
- **The review request** is one per review (`requests.review_id`, replacing `merge_request_id`): open while it waits on the CEO, resolved by Request changes, raised again by the lead's next `request_merge`, resolved once no merge request is left open.
- **Comments** (`review_comments`): on a line (repo, path, side `new`/`old`, line, the head it was written on, the line's text) or general; `draft` until Request changes sends them all in one message to the lead, then `sent` and kept. A sent comment on the new side follows its line as the lead's commits move it (`git diff -U0` from its head); one whose line changed is outdated. One on a removed line is outdated once its file changed.
- **API:** `GET /api/reviews`, `GET /api/reviews/:id`, `GET /api/reviews/:id/diff?since=1`, comments `POST` / `PUT` / `DELETE`, `POST …/changes {note}`, `POST …/merge {repos}`, `POST …/close {reason}` (the old Reject, kept as a secondary action, not to leave unwanted work open forever). The per-merge-request merge, reject and diff routes go.
- **UI:** a Reviews sidebar entry (count of those needing you) and page; the review page (file tree by repo, diff per file, line and general comments, the draft, the toggle, the decision bar); review cards (Dashboard, Needs you) and Repositories' rows link to it. `ReviewPanel`, `MergeDecision` and the diff dialog go.

## Steps

- [x] store: migration 005, `Reviews`, `ReviewComments`, `merge_requests.review_id` / `reviewed_head`, `requests.review_id`
- [x] git: compare two commits (`Compare`), heads
- [x] workflow: `request_merge` groups by review; review requests per review; Review read, comments, Request changes, Merge (repos), Close; line following and outdated
- [x] server routes; types
- [x] UI: Reviews list and page; cards link to it; old decision UI removed
- [x] specs: a [Review](../product/models/review.md) model and [Reviews](../product/features/reviews.md) feature spec; Merge request, Request, Pipeline and merges, Project pipeline, Dashboard, App shell, API, and the older mentions of Reject
- [ ] tried by the CEO in the browser on a real branch (only the unit tests and the build have run it)

**Done when:** a review opens from its row in the project's Inbox, with the file tree (by repo), the diff and comments; Request changes sends the draft to the lead in one message with each comment's repo, file and line; a review asked again shows the new diff, older comments on changed lines outdated; Merge to main works only from there; tests cover the comments' anchoring, the outdated rule and the message; specs: Requests, Project pipeline, Dashboard, Pipeline and merges, a Reviews feature spec, Merge request (comments).
