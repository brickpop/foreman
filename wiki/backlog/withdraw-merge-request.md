---
type: task
status: todo
priority: medium
tags: [feature, decision]
epic: workflow
---

# The lead withdraws a request to merge

**Why:** a review can stop making sense before the CEO decides it: another lead branch supersedes it (its work went into a bigger branch), or the lead dropped the approach. Today only the CEO can end it (Close, in [Reviews](../product/features/reviews.md)), so it stays in "Needs you" until they read it and close it by hand (the CEO, 2026-10-21).

## Proposal (to confirm)

- A lead tool, `withdraw_merge(branch, reason, superseded_by?)`, on its own open review only: its open merge requests end, its review request resolves (`withdrawn`), and it leaves "Needs you". The branch stays.
- On the review page: a status **Withdrawn**, with the lead's reason and, when given, a link to the review of `superseded_by`.
- The work items it carried are freed (no merge request) but stay `integrated`: their commits are still on the lead branch, and the next `request_merge` of a branch that contains them takes them. That's unlike Close, which sends them back to "in worktree".

## Open questions

- **A whole review, or some repos?** I recommend the whole review: a superseding branch replaces all of it. Withdrawing one repo is what Close does for the CEO, repo by repo.
- **A status of its own, or Closed?** I recommend Withdrawn: "the lead gave up on it" and "the CEO turned it down" read differently in history.
- **The CEO's draft comments on it:** they're lost when it's withdrawn. Should the CEO be told (a notice or a toast), or the drafts kept on the closed review?
- **Supersession done by Foreman:** when a new `request_merge` contains every commit of another open review of the same lead, should Foreman offer or do the withdrawal itself, or leave it to the lead?

**Done when:** the questions are answered, the tool is in [Team tools](../product/architecture/team-tools.md) and [Protocol](../product/architecture/protocol.md), and [Review](../product/models/review.md) has the status. Tests cover only the lead's own open review being withdrawn, the work items freed and taken by the next request, and the review request resolved.
