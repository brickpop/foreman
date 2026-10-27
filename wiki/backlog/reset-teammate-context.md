---
type: task
status: todo
priority: medium
tags: [feature]
epic: workflow
---

# Let the lead reset a teammate's context

**Why:** the CEO, 2026-10-26. A teammate's harness session lives as long as the deployment, and a restart continues the same conversation ([Agent runtime → Restarts](../product/architecture/agent-runtime.md#restarts)). So each new `ask` or hand-off lands on top of everything before it: context piles up, compaction kicks in, and old unrelated work leaks into new work. Continuing is fine when the new request follows the last one. After a long idle spell, or when the topic is new and unrelated, the teammate should start clean before the new instructions arrive.

**Done when:** `ask(teammate, request, fresh: true)` restarts the teammate in a fresh session with the request as its first message, and the [Team tools](../product/architecture/team-tools.md), [Protocol](../product/architecture/protocol.md), [Agent runtime](../product/architecture/agent-runtime.md) and [Harness adapters](../product/architecture/harness-adapters.md) specs describe it.

**Decided (the CEO, 2026-10-26):**
- **Shape: a flag on `ask`**, `ask(teammate, request, fresh: true)`, for the lead only. Resetting and delivering happen in one step, so nothing can arrive in between. There is no separate reset tool.
- **Mechanism: restart the harness without `--continue`.** It works the same on every harness and reuses the existing "starts fresh" path, including the `From Foreman: your worktrees hold earlier work…` notice ([Protocol](../product/architecture/protocol.md)). The fresh session gets the request as its opener. No slash command is typed into the TUI, so the adapters need no new capability.

**Still to decide with the CEO before building:**
- **Who decides when.** Only the lead, using its judgement, or a nudge from Foreman as well: `team()` already reports when a teammate's harness last reported, so it could show "idle for 3 h" and the lead's prompt could say when a reset makes sense. An automatic reset after N hours idle is an option, but it would drop context the lead may still want.
- **What survives a reset.** The worktrees and their uncommitted changes stay. Open requests to or from the teammate, a pending `awaiting` answer from the CEO, or a merge in progress: either refuse the reset while one of these is open, or go ahead and tell the teammate about them in its fresh session.
- **The CEO's view.** The terminal shows a new session (a new session id and log, as with a restart). Whether a marker or the Activity feed says "reset by the lead".

**Edge cases to cover:** resetting a busy teammate mid-turn (refuse, or wait until it's idle like a delivery does), resetting an asleep one (a [lazy start](../product/architecture/agent-runtime.md#lazy-starts) starts fresh anyway, so it's a no-op), a lead trying to reset itself or a teammate in another deployment, and the CEO typing a draft in the teammate's prompt at that moment ([Team tools → Delivery](../product/architecture/team-tools.md#delivery)).

- [ ] decide the questions above with the CEO, and write the answers into the specs
- [ ] the `fresh` flag on `ask`, the lead-only check, the restart without `--continue`, and its prompt guidance for the lead
- [ ] tests: reset then deliver lands in a clean session; worktrees untouched; refused while busy or for a non-teammate; continues as before without a reset
