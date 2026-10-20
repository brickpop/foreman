---
type: epic
status: done
order: 2
---

# Messaging and hand-offs

**Goal:** agents work together. `foreman-agent` and the agent link carry team-tool calls and hook events; `foreman-agent` is mounted into each container from Foreman's own binary (decided 2026-10-15, rather than built into the image: [Team tools → Transport](../../product/architecture/team-tools.md#transport)); agents `ask`, `reply`, `handoff` and `contribute`, and write git through `switch`, `commit` and `merge`; Messaging delivers to idle agents, waking an asleep one first ([Lazy starts](../../product/architecture/agent-runtime.md#lazy-starts)); the UI shows markers and the Messages drawer.

From the Claude Code adapter (epic 1 built `Launch`): `Deliver`, `Interpret` and `Diagnose`, shaped by [experiment D](../archive/prove-d-claude-delivery-hooks.md) (multi-line messages are not pasted; questions come through `AskUserQuestion` then `Notification`; `Stop` carries `last_assistant_message`). D's checks become the adapter's integration tests (`just test-claude`, needing the CEO's login), and `cmd/prove` and `cmd/proveagent` go.

**Hand-offs name a branch** (`handoff(mara, "wt/tove/LED-44", …)`): every agent has its own worktree since [Terminal multiplexer](./terminal-multiplexer.md), and all worktrees share the repo, so the lead sees the branch at once. Foreman checks the branch against the policy and delivers the hand-off; checks and work items arrive with the [Git](./git.md) epic. Agents work in parallel: there is no gate between agents of one project.

**Git through Foreman.** Each repo's `.git` is read-only in the containers, so `switch(branch)`, `commit(commits)`, `merge(branch)` and `abort_merge()` are team tools that Foreman runs on the host in the caller's worktrees (Git `Switch`, `CommitAll`, `Merge` and `AbortMerge`; `commit` also covers the live project repo for the lead, when named), and `handoff` names a committed branch, never committing itself ([Sandboxes and git policy → Agents and git](../../product/architecture/sandboxes-and-git.md#agents-and-git)). From here on agents branch and commit; the lead integrates a hand-off with `merge`, and catches up with a newer `main` with `merge("main")` (nothing is ever rebased).

**`contribute`** lets agents without write access to the knowledge base add to it: Foreman files the note in `project/wiki/inbox/` and tells the lead ([Protocol → `contribute`](../../product/architecture/protocol.md#contribute)).

**Usable when done:** the CEO gives the lead a goal, the lead delegates to teammates, teammates work and report back, and the lead reviews and continues, all visible in the UI.

**Journey steps:** 12–16 and 24 of [the journey](../../product/journey.md).

**Blocked by:** [Prove](./prove.md) (experiments C and D) and [Terminal multiplexer](./terminal-multiplexer.md).

**Decided in this epic (2026-10-16):** a lead's change request moves no work item; the author's next hand-off does ([Protocol → Work items and merge requests](../../product/architecture/protocol.md#work-items-and-merge-requests)).

**Exit criteria:** the steps above work end to end with the fake harness in tests and with Claude Code on both machines.
