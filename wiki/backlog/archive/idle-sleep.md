---
type: task
status: done
priority: high
tags: [feature]
epic: workflow
---

# Idle agents make room for those waiting for a slot

**Seen 2026-10-18:** with 7 agents and 6 slots, Lumen stayed `queued` behind five idle agents and its messages never arrived (the CEO read it as a delivery bug). The Activity panel now says why a message waits, but nothing frees the slot. Next after the Chief of Staff (the CEO, 2026-10-18).

**Why:** an agent that is done and waits for nothing still holds a container, a session slot (12 by default) and memory. Asleep, it holds none, and a message or the CEO's Start wakes it with its conversation ([Agent runtime → Lazy starts](../../product/architecture/agent-runtime.md#lazy-starts)). A clean exit (`/exit`) already does this (built 2026-10-16). What's left is Foreman doing it on its own.

**Decided (the CEO, 2026-10-16):**
- **A global setting, off by default.** When on, it applies after **20 minutes** of idle (the length is part of the setting).
- **Foreman asks the agent first** (proposed by the CEO, then judged noise the same day: see below). It types a line into the idle agent, something like `Are you currently doing anything or running any background process? If not, call idle()`. The agent answers by calling a new team tool **`idle()`**, and only then is it put to sleep. An agent that doesn't call it stays awake.

**Second thought (the CEO, 2026-10-16):** the ping is noise. It lands in the conversation, costs a turn, and stays there. The silent process check alone may be enough. Assess both when the card starts.

**Design** (from the 2026-10-16 discussion, to confirm when the card starts):
- **Foreman stops the container, never types `/exit`.** `/exit` is specific to Claude Code and could land on the CEO's unsent draft or a dialog. Stopping the container works for every harness, and the container is gone once a stop returns. A new instance-manager `Sleep(id)` marks the agent asleep itself, so the SIGTERM exit code isn't counted as a crash. Nothing is lost: the transcript is written as the agent goes, and `idle()` is called with no turn in progress.
- **A process check as a guard** (proposed): `foreman-agent`, already in the container, lists its processes. Foreman pings only when nothing is left but the harness and its own helpers, and `idle()` sleeps only then. This keeps a dev server the agent forgot from being killed, and spares the ping when it's pointless. The CEO found that an idle Claude Code container runs only `claude` (PID 1) and `foreman-agent mcp`; each harness adapter names its helpers.
- **The ping waits like any message:** for idle, and for the CEO's unsent draft (the countdown). Any activity before `idle()` cancels the sleep.

**Edge cases:**
- **A CEO shell in the container** (`podman exec`) is a process too, so it keeps the agent awake.
- **A lead waiting on a teammate's reply** sleeps too, and the reply wakes it.
- **A pane the CEO is watching** turns into the asleep view; the Line or Start wakes it.

**Decided (the CEO, 2026-10-23): the silent process check alone.** No ping, no `idle()` tool. Beyond the design above: input since the agent went idle restarts its clock, the terminal refuses input in the same step as the last check (so a delivery is never cut off by the stop), and the processes come from Podman's `top` (no code in `foreman-agent`) or `ps` on the Direct engine, so Direct agents sleep too. No adapter names extra helpers yet: only `foreman-agent` is one.

**Decided (the CEO, 2026-10-23, later): on demand, no setting.** The timer setting (`sleep_after_minutes`) was built, then replaced: a sleep after N minutes freed slots only by luck, and touched agents when slots were free. Now an idle agent sleeps only when an instance waits for its slot, the longest quiet first, after at least a minute idle. Built as [Agent runtime → Idle agents make room](../../product/architecture/agent-runtime.md#idle-agents-make-room).

- [x] on demand: when an instance waits for a slot (the setting went)
- [x] the process check, in the engines (`Processes`)
- [x] decide: the silent process check alone
- [x] the sleep in the instance manager; activity cancels a pending sleep
- [x] tests: nobody waiting, under a minute, a stray process, input, a draft, running and awaiting keep it awake; a helper and a zombie don't; a failed stop leaves it awake; the waiting one starts, and a wake starts it again
- [x] on the CEO's Podman: `just test-podman` passes `TestProcessesAreTheOthersAlive` (2026-10-23)
