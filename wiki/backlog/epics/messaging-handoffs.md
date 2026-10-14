---
type: epic
status: todo
order: 2
---

# Messaging and hand-offs

**Goal:** agents work together. `foreman-agent` and the agent link carry team-tool calls and hook events; agents `ask`, `reply` and `handoff`; Messaging delivers to idle agents; the UI shows markers and the Messages drawer.

**Hand-offs are messages only in this epic** ("done, see my changes"): every agent still works in the one shared project folder. Branches, worktrees and work items arrive with the [Git](./git.md) epic.

**Turns per project are part of this epic:** since the agents share the project folder, Messaging delivers to an agent only when no other agent of the same project is busy; the CEO's typing bypasses it with a warning ([Protocol → Messages](../../product/architecture/protocol.md#messages)). The Git epic removes the gate.

**Usable when done:** the CEO gives the lead a goal, the lead delegates to teammates, teammates work and report back, and the lead reviews and continues, all visible in the UI.

**Journey steps:** 12–16 (hand-offs as messages) and 24 of [the journey](../../product/journey.md).

**Blocked by:** [Prove](./prove.md) (experiments C and D) and [Terminal multiplexer](./terminal-multiplexer.md).

**Open decision to take in this epic:** whether a lead's change request moves a work item back to the author ([Protocol → Open questions](../../product/architecture/protocol.md#open-questions)). It gets a `decision` card when this epic's cards are written.

**Exit criteria:** the steps above work end to end with the fake harness in tests and with Claude Code on both machines.
