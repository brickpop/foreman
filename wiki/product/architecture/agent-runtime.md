---
type: spec
status: partial
tags: [architecture, runtime, sessions, agents]
---

# Agent runtime

How an [instance](../models/instance.md) (one agent of one [deployment](../models/deployment.md)) comes to life, runs, fails, stops and recovers. The **Instance manager** owns this ([Building blocks](./building-blocks.md#5-instance-manager-instances)): it asks the image builder for the project's image and the harness adapter for the launch spec, starts the process through the [execution engine](./session-runner.md), wraps it in a [Terminal](./building-blocks.md#2-terminal-terminal), and prepares the instance's worktrees through [Git](./sandboxes-and-git.md).

## Instance lifecycle

1. **Deploy**: create a deployment (see [Deploy a team](../features/deploy-team.md)), then an instance for each team member: the lead `starting` (or `queued`), every other member `asleep` ([Lazy starts](#lazy-starts)). Steps 2–3 happen when an instance starts, so an asleep member has no sandbox yet.
2. **Sandbox**: `Git.Prepare` makes a git worktree of each code repo at `<project>/.sandboxes/<agent>/<repo>` if it doesn't exist yet ([Sandboxes and git policy](./sandboxes-and-git.md#branches)), checked out on:
   - lead: `lead/<team>/integration` (created from `main` if missing)
   - members: their home branch `wt/<agent>/main` (created from `main`). They create `wt/<agent>/<topic>` when assigned a card.
3. **Compose prompt** ([Session prompt](./session-prompt.md)) and **spawn** the harness in its container with the sandbox as working directory ([Harness adapters](./harness-adapters.md)).
4. **Run**: status follows harness signals and team-message activity:
   - `running`: producing output / working a turn
   - `idle`: waiting at its prompt with nothing to do
   - `awaiting`: has an open [request](../models/request.md) to the CEO
   - `done`: handed off, nothing assigned
5. **Crash / exit / rate limit**: Foreman marks the instance `stopped` and raises a **blocked** request ([Requests](../features/requests.md)).
6. **Stop deployment**: terminate the processes (containers) and keep worktrees on disk; `Git.Prune` deletes the agents' `wt/*` branches already merged into `main` ([Branches](./sandboxes-and-git.md#branches)). Other deployments of the same team are not affected.
7. **Soul edit**: when a [team definition](../models/team-definition.md) changes, each running instance gets a context-update message on its next turn (injected at its next idle prompt, like a queued team message). A full re-prompt happens at the next restart.

Status values: `asleep | queued | starting | running | idle | awaiting | done | stopped`.

**Phasing:** in epic 1 there are no hooks yet (the agent link is epic 2), so statuses are the lifecycle's own: `asleep`, `queued`, `starting`, `running` (the process is alive), `stopped`.

### Lazy starts

A container runs only for an agent that has work (the CEO's decision, 2026-10-15): starting five containers when one agent is doing anything is waste.

- **On deploy only the lead starts.** Its harness starts with the goal as its **opening message**, `Your goal on <project>: <goal>`, and stays an interactive session ([Harness adapters](./harness-adapters.md): opener); the goal is in its session prompt too. Only on its first start: a restart continues the conversation and never gives the goal twice.
- **The other members are `asleep`:** an instance with no container, no sandbox yet, and no session slot.
- **Waking** an asleep instance queues it and starts it like any other (`POST /api/instances/:id/wake`). In epic 1 the CEO wakes one with **Start** in its pane ([Sessions](../features/sessions.md#panes)); from epic 2 a delivery to it (an `ask`, a handoff, a reply) wakes it first ([Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md)). Waking any other status does nothing.
- After a Foreman restart, an asleep instance stays asleep.
- **Later:** an agent that is done and waits for nothing goes back to `asleep`, its harness exited so that waking it resumes its conversation (native resume). It needs idle detection (epic 2) and resume per harness; see the backlog's *Not yet scheduled*.

### Status transitions

| From | Event | To |
|---|---|---|
| — | deployment created / agent hired | the lead on deploy: `queued` if no slot, else `starting`; every other member, and a hire, `asleep` |
| `asleep` | woken: the CEO's Start, or (epic 2) a delivery to it | `queued` if no slot, else `starting` |
| `queued` | a slot frees | `starting` |
| `starting` | sandbox ready and harness spawned | `running` (lead with a goal) or `idle` |
| `running` | harness idle signal ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)) | `idle`, or `done` if it handed off and has nothing assigned |
| `idle` / `done` | CEO input, team message or reply delivered | `running` |
| any live | `escalate(approval)` or blocking `escalate(question)` | `awaiting` |
| `awaiting` | request resolved | `running` |
| any live | process exit, crash, rate limit, no output for N min (default 20, see below), sandbox or git error (a merge conflict from `update` is not one: it goes back to the agent, who resolves it and commits) | `stopped` + blocked request; a crash first gets one automatic restart (see below) |
| `stopped` | CEO picks Retry / Switch harness | `starting` |
| any | deployment stopped | `stopped` (no request) |

## Concurrency

- Workspace setting `max running sessions` (default 12, 1–64; [Workspace settings](../models/workspace-settings.md)). Instances beyond the limit are `queued` and start, in creation order, as slots free up.
- A slot is held by every instance whose container is running, including one waiting on an approval or a reply. An `asleep` instance holds none. Releasing slots while waiting (stopping the container and resuming the harness session later) needs native resume, so it is revisited once the [Prove epic](../../backlog/epics/prove.md) shows which harnesses support it.
- The Chief of Staff session does not count towards the limit (it is Foreman's own, and the CEO must always be able to reach it).

## Recovery

Instances survive UI disconnects: the Terminal, not the browser, holds the process.

On server restart:
1. Load deployments and instances from `state.db` ([Persistence](./persistence.md)).
2. Ask the runner which containers are still running (`List`). Re-attach to each one whose instance is live; scrollback is replayed from Foreman's own session log, so nothing printed while Foreman was down is visible (an accepted gap, marked in the pane: [Session runner](./session-runner.md#podman-engine-production)).
3. Instances that should be live but whose container is gone are restarted (see Restarts below).
4. Containers that belong to no live instance are orphans: stopped and reported in the server log.

## Restarts

- **How:** when the adapter supports it, the harness resumes its own session. Claude Code: `claude --continue`, the latest conversation in the instance's private config dir, which is always its own (no session id needed); skipped when there is none yet. Others by their session id, stored on the [instance](../models/instance.md) (Codex session resume). Otherwise it starts fresh with a summary: the last task note (`instances.task`) plus the instance's last team messages.
- **When, automatically:** a crash gets **one** automatic restart (counted in `instances.restarts`) before a blocked request is raised; a second crash, or any rate limit, goes straight to the CEO. A rate limit is never retried automatically.
- **Stuck sessions:** "no output for N minutes" (`stuck_after_minutes`, default 20, a [workspace setting](../models/workspace-settings.md)) counts silence only while the harness is not signalling a running tool through its hooks, so a long, silent test run isn't "stuck".

## Memory

- The soul (personality, job, skills) is shared across deployments ([Agent soul](../models/agent-soul.md)).
- Working memory (notes, scratch files, harness-native memory) is **per instance** and stays inside that sandbox / container. Nothing leaks between projects.
- Long-lived project knowledge belongs in the wiki.
