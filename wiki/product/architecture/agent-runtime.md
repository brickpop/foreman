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
- After a Foreman restart, an asleep instance stays asleep. The restore itself waits until Podman first answers ([First run](../features/first-run.md#foreman-always-starts)): Foreman serves before that.
- **Later:** an agent that is done and waits for nothing goes back to `asleep`, its harness exited so that waking it resumes its conversation (native resume). It needs idle detection (epic 2) and resume per harness; see the [Idle sleep](../../backlog/idle-sleep.md) card (a setting, off by default; Foreman asks the agent, which calls `idle()`).

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
| any live | process exit, crash, rate limit, no output for N min (default 20, see below), sandbox or git error (a merge conflict from `merge` is not one: it goes back to the agent, who resolves it and commits) | `stopped` + blocked request; a crash first gets one automatic restart (see below) |
| `stopped` | CEO picks Retry (or, later, Switch profile) | `starting` |
| any | deployment stopped | `stopped` (no request) |

## Concurrency

- Workspace setting `max running sessions` (default 12, 1–64; [Workspace settings](../models/workspace-settings.md)). Instances beyond the limit are `queued` and start, in creation order, as slots free up.
- A slot is held by every instance whose container is running, including one waiting on an approval or a reply. An `asleep` instance holds none. Releasing slots while waiting (stopping the container and resuming the harness session later) needs native resume, so it is revisited once the [Prove epic](../../backlog/epics/prove.md) shows which harnesses support it.
- The Chief of Staff session does not count towards the limit (it is Foreman's own, and the CEO must always be able to reach it).

## Recovery

Instances survive UI disconnects: the Terminal, not the browser, holds the process.

**When Foreman exits cleanly** (Ctrl-C, SIGTERM; the CEO's decision, 2026-10-16), it stops serving, then stops every instance's container (and an open login terminal), all at once, each with the usual grace (SIGTERM, then a kill after 10 s). While Foreman is gone nothing reads their output, answers their team tools or delivers their messages, so a running container would only hold memory and may spend tokens on a turn nobody follows. Each instance goes **asleep**, not `stopped`, with its restart count untouched; queued ones too. One whose stop fails (Podman not answering) keeps its status, so the next start re-attaches it, as after a crash. Nothing starts or wakes during the stop (deploy and wake refuse with `NOT_READY`). On the next start they stay asleep until work or the CEO's Start wakes them, continuing their conversation; an agent stopped mid-turn loses only that turn's unfinished part. Messages waiting at exit wake their agent on the next start, through the startup delivery. A second Ctrl-C during the stop ends Foreman at once and leaves the rest running, for the next start to re-attach. The Podman machine (macOS), and a Linux Podman service the CEO started, stay up; one Foreman started ends with it.

After a crash (or that second Ctrl-C), containers are still running. On the next start:
1. Load deployments and instances from `state.db` ([Persistence](./persistence.md)).
2. Ask the runner which containers are still running (`List`), and keep only this workspace's: every container is labelled `foreman.workspace` with the workspace's path, and named with a short hash of it (`foreman-19269e-mara-ledger`), because every workspace on the machine shares one Podman and two may both have a `mara@ledger` (found 2026-10-16: a second workspace stopped the first one's agents). Re-attach to each one whose instance is live; scrollback is replayed from Foreman's own session log, so nothing printed while Foreman was down is visible (an accepted gap, marked in the pane: [Session runner](./session-runner.md#podman-engine-production)).
3. Instances that should be live but whose container is gone are restarted (see Restarts below).
4. This workspace's containers that belong to no live instance are orphans: stopped and reported in the server log. Another workspace's are never touched. Containers from before the label existed are adopted when one of this workspace's instances matches them, and otherwise left alone.

## Restarts

- **How:** when the adapter supports it, the harness resumes its own session. Claude Code: `claude --continue`, the latest conversation in the instance's private config dir, which is always its own (no session id needed). That dir outlives deployments, so a deployment's first start writes down the conversations already there (`foreman-fresh-start`), and a restart continues only one that isn't on that list; with none, it starts fresh (found 2026-10-16: it would have picked up the previous deployment's conversation). A session that starts fresh gets the lead's goal as its opener; one that continues doesn't get it twice. Others by their session id, stored on the [instance](../models/instance.md) (Codex session resume). Otherwise it starts fresh with a summary: the last task note (`instances.task`) plus the instance's last team messages.
- **A clean exit is not a crash** (the CEO's decision, 2026-10-16): a harness that ends with code 0 (the CEO's `/exit`, say) goes **asleep**, holding no slot, and wakes like any sleeper, on a message or the CEO's Start, picking up its conversation. Messages already waiting for it wake it at once.
- **When, automatically:** a crash gets **one** automatic restart (counted in `instances.restarts`) before a blocked request is raised; a second crash, or any rate limit, goes straight to the CEO. A rate limit is never retried automatically. A stopped instance of an active deployment starts again on the CEO's **Start again** (its pane), with its one automatic restart back; messages never revive it, so a crash can't loop.
- **Stuck sessions:** "no output for N minutes" (`stuck_after_minutes`, default 20, a [workspace setting](../models/workspace-settings.md)) counts silence only while the harness is not signalling a running tool through its hooks, so a long, silent test run isn't "stuck".

## Memory

- The soul (personality, job, skills) is shared across deployments ([Agent soul](../models/agent-soul.md)).
- Working memory (notes, scratch files, harness-native memory) is **per instance** and stays inside that sandbox / container. Nothing leaks between projects.
- Long-lived project knowledge belongs in the wiki.
