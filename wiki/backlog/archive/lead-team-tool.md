---
type: task
status: done
priority: medium
tags: [feature]
epic: workflow
---

# The lead sees where its teammates stand: `team()`

**Why:** the lead couldn't tell who is doing what. Its only way to find out was an `ask`, which interrupts the teammate, waits on a reply, and wakes an asleep member for nothing. Foreman already knows each instance's status and each worktree's branch (the CEO, 2026-10-23).

## Decided (the CEO, 2026-10-23)

- **A read-only tool, lead only:** `team()`, beside `reviews()` ([Protocol → `team`](../../product/architecture/protocol.md#team)). Members get their work from the lead and don't watch each other.
- **Pull only:** the lead calls it when it wants to know. No notices when a teammate's status changes.
- **One line per teammate:**
  - Its [status](../../product/models/instance.md#statuses), with Foreman's stop reason when it has one.
  - When its harness last reported: the activity line's time, the one the Dashboard shows. It is kept in memory, so it is missing until the harness reports again after a restart. Nothing new is stored.
  - Each code repo's checked-out branch, and whether it holds uncommitted changes or a merge under way.
- **Never shown:** the activity line's text, the terminal, or anything else from inside a teammate's sandbox.
- **Calling it wakes nobody:** an asleep member's worktrees are read on the host. The worktree lookup is shared with the start notice (`worktreesIn`), so there is one way to find them.

## Done (2026-10-23)

- `internal/workflow/team.go`, and its line in the lead's prompt.
- Tests in `internal/workflow/team_test.go` and `internal/prompt/prompt_test.go` cover:
  - each part of a line;
  - the activity line's text not crossing;
  - a merge under way;
  - a stop reason;
  - no time for an asleep member;
  - a member added since the deploy;
  - a lead alone;
  - members refused, and the tool left out of their prompt.
