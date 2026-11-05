---
type: task
status: done
priority: low
tags: [feature]
epic: workflow
---

# A review comment follows its file when the file is renamed

**Why:** the CEO, 2026-11-04. A line comment is stored with the file's path, and kept in place by comparing its commit with the branch head on that path only (`follow` in `internal/workflow/reviews.go`, `Git.Compare(…, c.Path)`). If the lead later renames the file, that path is deleted in the newer diff: the comment lands under **Outdated** or **Not in this view** ([Reviews → A review's page](../../product/features/reviews.md#a-reviews-page)), even when its line didn't change, and the lead is told about a path that no longer exists.

**Done when:** a comment on a file renamed since (with or without edits) shows on the renamed file, at its line now, and the lead gets the new path; a comment whose file was deleted, or whose line changed, is still outdated. [Reviews](../../product/features/reviews.md) says so.

**Decided (2026-11-05):** only shown there, worked out on each read as its line is (`nowPath` beside `now`); the stored path stays as written. Old-side comments (a removed line of the target) don't follow: their file is the target's, which a rename on the branch doesn't change.

- [x] find the rename between the comment's commit and the head (a `--find-renames` diff over the whole change, not only the comment's path), and place the comment on the new path
- [x] tests (`TestACommentFollowsARenamedFile`, `TestRenamed`, `placeComments`): rename only, rename with an edit above the line (the line moves), rename with the line itself changed (outdated), a file renamed twice, a file deleted
