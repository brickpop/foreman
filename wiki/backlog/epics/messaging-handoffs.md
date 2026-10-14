---
type: epic
status: todo
order: 2
---

# Messaging and hand-offs

**Goal:** agents work together. `foreman-agent` and the agent link carry team-tool calls and hook events; `foreman-agent` is added to the agent image ([Image builder](../image-builder.md)); agents `ask`, `reply`, `handoff` and `contribute`, and write git through `switch`, `commit` and `update`; Messaging delivers to idle agents; the UI shows markers and the Messages drawer.

**Hand-offs name a branch** (`handoff(mara, "wt/tove/LED-44", …)`): every agent has its own worktree since [Terminal multiplexer](./terminal-multiplexer.md), and all worktrees share the repo, so the lead sees the branch at once. Foreman checks the branch against the policy and delivers the hand-off; checks and work items arrive with the [Git](./git.md) epic. Agents work in parallel: there is no gate between agents of one project.

**Git through Foreman.** Each repo's `.git` is read-only in the containers, so `switch(branch)`, `commit(message)` and `update(branch)` are team tools that Foreman runs on the host in the caller's worktrees (Git `Switch`, `Commit` and `Update`; `commit` also covers the live project repo for the lead), and `handoff` commits pending work first ([Sandboxes and git policy → Agents and git](../../product/architecture/sandboxes-and-git.md#agents-and-git)). From here on agents branch and commit; the lead integrates a hand-off with `update`, and catches up with a newer `main` with `update("main")` (nothing is ever rebased).

**`contribute`** lets agents without write access to the knowledge base add to it: Foreman files the note in `project/wiki/inbox/` and tells the lead ([Protocol → `contribute`](../../product/architecture/protocol.md#contribute)).

**Usable when done:** the CEO gives the lead a goal, the lead delegates to teammates, teammates work and report back, and the lead reviews and continues, all visible in the UI.

**Journey steps:** 12–16 and 24 of [the journey](../../product/journey.md).

**Blocked by:** [Prove](./prove.md) (experiments C and D) and [Terminal multiplexer](./terminal-multiplexer.md).

**Open decision to take in this epic:** whether a lead's change request moves a work item back to the author ([Protocol → Open questions](../../product/architecture/protocol.md#open-questions)). It gets a `decision` card when this epic's cards are written.

**Exit criteria:** the steps above work end to end with the fake harness in tests and with Claude Code on both machines.
