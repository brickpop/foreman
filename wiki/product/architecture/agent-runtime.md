---
type: spec
status: planned
tags: [architecture, runtime, sessions, agents]
---

# Agent runtime

How an [instance](../models/instance.md) (one agent of one [deployment](../models/deployment.md)) comes to life, runs, fails, stops and recovers. The session supervisor owns this; it starts processes through the [session runner](./session-runner.md) and prepares their working copies through the [sandbox manager](./sandboxes-and-git.md).

## Instance lifecycle

1. **Deploy**: create a deployment (see [Deploy a team](../features/deploy-team.md)), then for each team member create an instance in the `starting` state.
2. **Sandbox**: clone each project repo into `<project>/.sandboxes/<agent>/<repo>`. Fetch, then check out:
   - lead: `lead/integration` (created from `main` if missing; naming is an open question in [Sandboxes and git policy](./sandboxes-and-git.md#open-questions))
   - members: idle on `main`. They create `wt/<agent>/<card>` when assigned a card.
3. **Compose prompt** ([Session prompt](./session-prompt.md)) and **spawn** the harness in its container with the sandbox as working directory ([Harness adapters](./harness-adapters.md)).
4. **Run**: status follows harness signals and ACP activity:
   - `running`: producing output / working a turn
   - `idle`: waiting at its prompt with nothing to do
   - `awaiting`: has an open [request](../models/request.md) to the CEO
   - `done`: handed off, nothing assigned
5. **Crash / exit / rate limit**: Foreman marks the instance `stopped` and raises a **blocked** request ([Requests](../features/requests.md)).
6. **Stop deployment**: terminate the processes (containers) and keep sandboxes and worktrees on disk. Other deployments of the same team are not affected.
7. **Soul edit**: when a [team definition](../models/team-definition.md) changes, each running instance gets a context-update message on its next turn (injected at its next idle prompt, like a queued ACP message). A full re-prompt happens at the next restart.

Status values: `starting | running | idle | awaiting | done | stopped | queued`.

### Status transitions

| From | Event | To |
|---|---|---|
| — | deployment created / agent hired | `queued` if no slot, else `starting` |
| `queued` | a slot frees | `starting` |
| `starting` | sandbox ready and harness spawned | `running` (lead with a goal) or `idle` |
| `running` | harness idle signal ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)) | `idle`, or `done` if it handed off and has nothing assigned |
| `idle` / `done` | CEO input, ACP message or reply delivered | `running` |
| any live | `escalate(approval)` or blocking `escalate(question)` | `awaiting` |
| `awaiting` | request resolved | `running` |
| any live | process exit, crash, rate limit, no output for N min (default 20), sandbox or git error, merge conflict on `lead/*` | `stopped` + blocked request |
| `stopped` | CEO picks Retry / Switch harness | `starting` |
| any | deployment stopped | `stopped` (no request) |

## Concurrency

- Workspace setting `max running sessions` (default 12, 1–64; [Workspace settings](../models/workspace-settings.md)). Instances beyond the limit are `queued` and start, in creation order, as slots free up.
- A slot is held by every instance whose container is running. Requesters waiting on a reply don't hold a slot when the harness supports pause or resume (see open questions).
- The Chief of Staff session does not count towards the limit (it is Foreman's own, and the CEO must always be able to reach it).

## Recovery

Instances survive UI disconnects: the supervisor, not the browser, owns the attach.

On server restart:
1. Load deployments and instances from `state.db` ([Persistence](./persistence.md)).
2. Ask the runner which containers are still running (`List`). Re-attach to each one whose instance is live; scrollback is replayed from Foreman's own session log, so nothing printed while Foreman was down is visible unless the runner can replay it (see [Session runner](./session-runner.md#open-questions)).
3. Instances that should be live but whose container is gone are restarted "with the last task note" (`instances.task`), or resumed via the harness's native session resume where supported (open question 3).
4. Containers that belong to no live instance are orphans: stopped and reported in the server log.

## Memory

- The soul (personality, job, skills) is shared across deployments ([Agent soul](../models/agent-soul.md)).
- Working memory (notes, scratch files, harness-native memory) is **per instance** and stays inside that sandbox / container. Nothing leaks between projects.
- Long-lived project knowledge belongs in the wiki.

## Open questions

1. **Pausing awaiting requesters.** Which harnesses support pause/resume so that an instance blocked on `escalate(approval)` or waiting on `ask` releases its slot? Options: stop the container and resume the harness session later (needs native resume), freeze it (`podman pause`, keeps memory, frees CPU but not the "slot" semantics), or always hold the slot. Recommendation: hold the slot in v1, revisit after the harness spikes.
2. **Silence threshold.** "No output for N min (default 20)" raises a blocked request. A long test run is silent but healthy. Recommendation: count silence only while the harness is not signalling a running tool (when hooks exist), and make N a workspace setting.
3. **Restart semantics** (PRD §16.8): resume harness-native session state where supported (`claude --resume <id>`, Codex session resume), or start fresh with a summary? Recommendation: native resume when the adapter supports it (store the harness session id on the instance), else fresh start with the last task note plus the last N ACP messages of that instance.
4. **Retry policy.** Does Foreman auto-restart a crashed harness before raising a blocked request (e.g. once, with backoff), or always ask the CEO? `instances.restarts` exists in the model. Recommendation: one automatic restart for crashes, never for rate limits.
