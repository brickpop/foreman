---
type: spec
status: partial
tags: [architecture, runtime, sessions, agents]
---

# Agent runtime

How an [instance](../models/instance.md) (one agent of one [deployment](../models/deployment.md)) comes to life, runs, fails, stops and recovers. The **Instance manager** owns this ([Building blocks](./building-blocks.md#5-instance-manager-instances)): it asks the image builder for the project's image and the harness adapter for the launch spec, starts the process through the [execution engine](./session-runner.md), wraps it in a [Terminal](./building-blocks.md#2-terminal-terminal), and prepares the instance's worktrees through [Git](./sandboxes-and-git.md).

## Instance lifecycle

1. **Deploy**: create a deployment (see [Deploy a team](../features/deploy-team.md)), then an instance for each team member: the lead `starting` (or `queued`), every other member `asleep` ([Lazy starts](#lazy-starts)). Steps 2–3 happen when an instance starts, so an asleep member has no sandbox yet.
2. **Sandbox**: a project with a [repo problem](../models/project-config.md#the-project-record) (a code repo missing or changed) refuses the start. `Git.Prepare` makes a git worktree of each code repo at `<workspace>/agents/<agent>/<repo>` if it doesn't exist yet, and brings the agent's home branch up to `main` when it is idle ([Sandboxes and git policy](./sandboxes-and-git.md#branches)), checked out on:
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
7. **Soul edit**: when a [team definition](../models/team-definition.md) changes, nothing running is touched: each instance gets the new prompt at its next start, or at once when the CEO reloads the deployment ([Team definition → Reload](../models/team-definition.md#reload)). A member hired into an active deployment gets its instance, asleep.

Status values: `asleep | queued | starting | running | idle | awaiting | done | stopped`.

**Phasing:** in epic 1 there are no hooks yet (the agent link is epic 2), so statuses are the lifecycle's own: `asleep`, `queued`, `starting`, `running` (the process is alive), `stopped`.

### Lazy starts

A container runs only for an agent that has work (the CEO's decision, 2026-10-15): starting five containers when one agent is doing anything is waste.

- **On deploy only the lead starts**, with no opening message: the CEO writes first, in its Session (the CEO, 2026-10-31; until then the goal was typed in as its first message). The goal is the project's, in its wiki, which its [session prompt](./session-prompt.md) points to.
- **The other members are `asleep`:** an instance with no container, no sandbox yet, and no session slot.
- **Waking** an asleep instance queues it and starts it like any other (`POST /api/instances/:id/wake`). In epic 1 the CEO wakes one with **Start** in its session ([Sessions](../features/sessions.md#the-terminal)); from epic 2 a delivery to it (an `ask`, a handoff, a reply) wakes it first ([Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md)). Waking any other status does nothing.
- After a Foreman restart, an asleep instance stays asleep. The restore itself waits until Podman first answers ([First run](../features/first-run.md#foreman-always-starts)): Foreman serves before that.
- **An idle agent goes back to sleep** when another instance waits for its slot: [Idle agents make room](#idle-agents-make-room).

### Status transitions

| From | Event | To |
|---|---|---|
| — | deployment created / agent hired | the lead on deploy: `queued` if no slot, else `starting`; every other member, and a hire, `asleep` |
| `asleep` | woken: the CEO's Start, or (epic 2) a delivery to it | `queued` if no slot, else `starting` |
| `queued` | a slot frees | `starting` |
| `starting` | sandbox ready and harness spawned | `running` or `idle` |
| `running` | harness idle signal ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)) | `idle`, or `done` if it handed off and has nothing assigned |
| `idle` / `done` | CEO input, team message or reply delivered | `running` |
| `idle` | another instance waits for a slot; quiet a minute, with nothing else running ([Idle agents make room](#idle-agents-make-room)) | `asleep` |
| any live | `escalate(approval)` or blocking `escalate(question)` | `awaiting` |
| `awaiting` | request resolved | `running` |
| any live | process exit, crash, rate limit, no output for N min (default 20, see below), sandbox or git error (a merge conflict from `merge` is not one: it goes back to the agent, who resolves it and commits) | `stopped` + blocked request; a crash first gets one automatic restart (see below) |
| `stopped` | CEO picks Retry (or, later, Switch profile) | `starting` |
| any | deployment stopped | `stopped` (no request) |

## Concurrency

- Global setting `max running sessions` (default 12, 1–64; [Global settings](../models/global-settings.md)). Instances beyond the limit are `queued` and start, in creation order, as slots free up. Only an instance that actually waits shows `queued` (one that gets a slot at once goes straight to `starting`; the CEO, 2026-10-18: a passing `queued` warned them about slots for nothing). Raising the limit starts those that now fit at once (any change to `settings.toml`, from the UI or an editor); lowering it stops nothing.
- A slot is held by every instance whose container is running, including one waiting on an approval or a reply. An `asleep` instance holds none. An idle one gives its slot to an instance that waits by going to sleep ([Idle agents make room](#idle-agents-make-room)); one waiting on an approval keeps it, since stopping it would lose its open dialog.
- The Chief of Staff's session does not count towards the limit: it isn't an agent of a deployment, and the CEO must always be able to reach it ([Chief of Staff](../features/chief-of-staff.md#how-it-runs)).

## Recovery

Instances survive UI disconnects: the Terminal, not the browser, holds the process.

**When Foreman exits cleanly** (Ctrl-C, SIGTERM; the CEO's decision, 2026-10-16), it stops serving, then stops every instance's container (and an open login terminal), all at once, each with the usual grace (SIGTERM, then a kill after 10 s). While Foreman is gone nothing reads their output, answers their team tools or delivers their messages, so a running container would only hold memory and may spend tokens on a turn nobody follows. Each instance goes **asleep**, not `stopped`, with its restart count untouched; queued ones too. One whose stop fails (Podman not answering) keeps its status, so the next start re-attaches it, as after a crash. Nothing starts or wakes during the stop (deploy and wake refuse with `NOT_READY`). On the next start they stay asleep until work or the CEO's Start wakes them, continuing their conversation; an agent stopped mid-turn loses only that turn's unfinished part. Messages waiting at exit wake their agent on the next start, through the startup delivery. A second Ctrl-C during the stop ends Foreman at once and leaves the rest running, for the next start to re-attach. The Podman machine (macOS), and a Linux Podman service the CEO started, stay up; one Foreman started ends with it.

After a crash (or that second Ctrl-C), containers are still running. On the next start:
1. Load deployments and instances from `state.db` ([Persistence](./persistence.md)).
2. Ask the runner which containers are still running (`List`), and keep only this home's: every container is labelled `foreman.home` with the home's path, and named with a short hash of it (`foreman-19269e-mara-ledger`), because every Foreman on the machine (the CEO's, `just dev`'s, a test's: one per home) shares one Podman and two may both have a `mara@ledger` (found 2026-10-16: a second Foreman stopped the first one's agents). Re-attach to each one whose instance is live; scrollback is replayed from Foreman's own session log, so nothing printed while Foreman was down is visible (an accepted gap, marked in the pane: [Session runner](./session-runner.md#podman-engine-production)).
3. Instances that should be live but whose container is gone are restarted (see Restarts below).
4. This home's containers that belong to no live instance are orphans: stopped and reported in the server log. Containers without this home's label (another home's, or unlabelled) are never touched nor adopted.

## Restarts

- **How:** when the adapter supports it, the harness resumes its own session. Claude Code: `claude --continue`, the latest conversation in the instance's private config dir, which is always its own (no session id needed). That dir is per agent and project and outlives deployments. Today a deployment's first start writes down the conversations already there (`foreman-fresh-start`), and a restart continues only one that isn't on that list; with none, it starts fresh (found 2026-10-16: it would have picked up the previous deployment's conversation). So a team stopped and deployed again loses its agents' conversations, still on disk: **a bug** (the CEO, 2026-11-02). Target: a redeploy continues each agent's last conversation on the project, unless the CEO picks **Start fresh** in [Deploy a team](../features/deploy-team.md) ([Continue conversations on redeploy](../../backlog/continue-conversations-on-redeploy.md)). Agent ids are unique within a team only, so the dir may hold an earlier team's agent of the same id: its conversations are never continued by another team's ([Team definition](../models/team-definition.md#schema)). A session that starts fresh gets the lead's goal as its opener; one that continues doesn't get it twice. Others by their session id, stored on the [instance](../models/instance.md) (Codex session resume). Otherwise it starts fresh with a summary: the last task note (`instances.task`) plus the instance's last team messages.
- **Another harness starts fresh** (the CEO, 2026-10-18): a start on another harness than the instance's last run (the project's profile changed) can't continue that conversation, so it starts fresh. Another profile on the same harness (another account) continues.
- **The CEO's Restart** (Project settings, for agents still on another profile): stops a running instance and starts it again at once, on the project's profile as it is now, with its one automatic restart back. One that isn't up is left as it is.
- **A clean exit is not a crash** (the CEO's decision, 2026-10-16): a harness that ends with code 0 (the CEO's `/exit`, say) goes **asleep**, holding no slot, and wakes like any sleeper, on a message or the CEO's Start, picking up its conversation. Messages already waiting for it wake it at once.
- **When, automatically:** a crash gets **one** automatic restart (counted in `instances.restarts`) before a blocked request is raised; a second crash, or any rate limit, goes straight to the CEO. A rate limit is never retried automatically. A stopped instance of an active deployment starts again on the CEO's **Start again** (its pane), with its one automatic restart back; messages never revive it, so a crash can't loop.

## Stuck agents

An agent is **stuck** when it is mid-turn (`running`) and **no hook event at all** has come from it for `stuck_after_minutes` (default 20, a [global setting](../models/global-settings.md), read at each check, so a change applies at once). Hooks fire on every prompt, every tool call (Claude Code: before and after each; Pi: as each starts) and every turn end ([Harness adapters → Hooks](./harness-adapters.md#idle-and-waiting-detection)), so a working agent sends one every few seconds to a minute (the CEO, 2026-10-20).

- **One rule for every harness, long tools included:** Pi has no "tool finished" event, so a long tool and long thinking after a tool look the same; a test run that genuinely takes 25 minutes is flagged, and the CEO dismisses it.
- **Not terminal output:** Claude Code's spinner redraws the screen the whole time it thinks, so a hung turn never goes quiet on screen.
- **Idle and awaiting agents are never stuck:** they are waiting, not working.
- **What happens:** the Instance manager checks once a minute and raises one **blocked** [request](../features/requests.md) per stuck agent, "<Name> may be stuck: no activity for <N> min.", with **Restart** (and the card opens the session). The agent keeps running; nothing is restarted for the CEO. The card resolves itself on the agent's next hook event (whatever it says), when its run ends (a restart, a stop, a crash), or when Foreman restarts (its watch starts over, and raises it again if the agent is still quiet).
- **When activity was last heard** is kept in the manager's memory, never stored: a Foreman restart starts every agent's watch over.

## Idle agents make room

An **idle** agent holds a container, a session slot and memory while it waits for nothing. When another instance waits for a slot (`queued`), the idle agent quiet the longest gives its slot up: Foreman puts it to sleep, and the waiting one starts (the CEO, 2026-10-16 and 2026-10-23). Asleep, it holds nothing, and a message or the CEO's Start wakes it with its conversation ([Lazy starts](#lazy-starts)), as after a clean exit. With free slots, no agent is touched: there is no setting.

- **One per waiting instance**, the longest quiet first; an agent that isn't quiet is passed over for the next.
- **Silently.** Foreman asks the agent nothing: a question typed into its session would cost a turn and stay in its conversation (the CEO judged it noise). Instead it checks what runs.
- **Only when nothing else runs.** The [engine](./session-runner.md) lists the session's processes besides the harness itself: in a container, all but its PID 1 (Podman's `top`, read from `/proc`, nothing installed in the image); on the Direct engine, the harness's descendants (`ps`). Zombies (finished, never reaped: the harness, as PID 1, reaps only its own children) don't count. Anything but `foreman-agent` (the team tools' MCP server, which Claude Code and Pi keep running) keeps the agent awake: a dev server, a watcher or a build it started and forgot, and a shell the CEO opened in its container.
- **Quiet means:** `idle` for at least a minute (one just finished may be about to get its reply; `running` and `awaiting` never sleep: an open dialog would be lost), no input since (from the CEO or a delivery: the minute starts over at the last keystroke), and no draft of the CEO's left unsent in its prompt.
- **How:** the Instance manager looks when instances are left waiting, and once a minute while they wait (for agents that have gone idle since). Its terminal is closed to input in the same step as the last check, so a delivery either came first (and the agent stays awake) or is refused and waits for the wake, which the sleep then starts at once. Foreman then stops the container, never types `/exit` (a Claude Code command, which could land on a dialog): SIGTERM, a kill after 10 s. A stop that fails leaves the agent awake, taking input again. Nothing is lost: harnesses write their transcript as they go, and an idle agent has no turn in progress.
- **A pane the CEO is watching** turns into the asleep view; Start wakes it.
- **When it went idle** is kept in the manager's memory: after a Foreman restart, the minute of an agent still idle starts over.
- **Not covered:** a harness waiting on a timer of its own (a scheduled wake-up, no process) is idle and sleeps, losing the timer. A process that detached itself from the harness on the Direct engine (a double fork) is no longer its descendant and doesn't keep it awake; in a container it still does.


## Memory

- The soul (personality, job, skills) is shared across deployments ([Agent soul](../models/agent-soul.md)).
- Working memory (notes, scratch files, harness-native memory) is **per instance** and stays inside that sandbox / container. Nothing leaks between projects.
- Long-lived project knowledge belongs in the wiki.
