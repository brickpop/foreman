---
type: epic
status: in-progress
order: 4
---

# Workflow with the CEO

**Goal:** everything that needs the CEO reaches them, and their answers flow back. Requests (approval, question, waiting in its session, blocked, finding) on the Dashboard and Changes with notifications; `request_merge` runs the checks and raises the review; the CEO merges to main, creates a PR on the forge, or rejects; merges are noticed and other leads are told to merge `main` in.

**Progress:** `request_merge`, Merge to main and Reject landed with the [Git epic](./git.md); requests from `escalate`, notifications and the rest are still to do.

**Usable when done:** the full loop from a goal to a merge into `main`, with approvals, crashes and findings handled from the UI.

**Journey steps:** 18–19 and 21–23 of [the journey](../../product/journey.md).

**Blocked by:** [Messaging and hand-offs](./messaging-handoffs.md), and [Git](./git.md) for merges (local merge, the optional forge PR, integration).

**Exit criteria:** the steps above work end to end, including a Merge to main, and a real PR on GitHub (and Gitea/Forgejo) created and merged by the CEO.
