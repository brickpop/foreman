---
type: task
status: todo
priority: high
tags: [feature]
epic: workflow
---

# Put idle agents to sleep (a setting, off by default)

**Seen 2026-10-18:** with 7 agents and 6 slots, Lumen stayed `queued` behind five idle agents and its messages never arrived (the CEO read it as a delivery bug). The Activity panel now says why a message waits, but nothing frees the slot. Next after the Chief of Staff (the CEO, 2026-10-18).

**Why:** an agent that is done and waits for nothing still holds a container, a session slot (12 by default) and memory. Asleep, it holds none, and a message or the CEO's Start wakes it with its conversation ([Agent runtime → Lazy starts](../product/architecture/agent-runtime.md#lazy-starts)). A clean exit (`/exit`) already does this (built 2026-10-16). What's left is Foreman doing it on its own.

**Decided (the CEO, 2026-10-16):**
- **A global setting, off by default.** When on, it applies after **20 minutes** of idle (the length is part of the setting).
- **Foreman asks the agent first** (proposed by the CEO, then judged noise the same day: see below). It types a line into the idle agent, something like `Are you currently doing anything or running any background process? If not, call idle()`. The agent answers by calling a new team tool **`idle()`**, and only then is it put to sleep. An agent that doesn't call it stays awake.

**Second thought (the CEO, 2026-10-16):** the ping is noise. It lands in the conversation, costs a turn, and stays there. The silent process check alone may be enough. Assess both when the card starts.

**Design** (from the 2026-10-16 discussion, to confirm when the card starts):
- **Foreman stops the container, never types `/exit`.** `/exit` is specific to Claude Code and could land on the CEO's unsent draft or a dialog. Stopping the container works for every harness, and the container is gone once a stop returns. A new instance-manager `Sleep(id)` marks the agent asleep itself, so the SIGTERM exit code isn't counted as a crash. Nothing is lost: the transcript is written as the agent goes, and `idle()` is called with no turn in progress.
- **A process check as a guard** (proposed): `foreman-agent`, already in the container, lists its processes. Foreman pings only when nothing is left but the harness and its own helpers, and `idle()` sleeps only then. This keeps a dev server the agent forgot from being killed, and spares the ping when it's pointless. The CEO found that an idle Claude Code container runs only `claude` (PID 1) and `foreman-agent mcp`; each harness adapter names its helpers.
- **The ping waits like any message:** for idle, and for the CEO's unsent draft (the countdown). Any activity before `idle()` cancels the sleep.

**Edge cases:**
- **The ping stays in the conversation:** it costs a turn (tokens), and the agent's context keeps it after waking.
- **An agent that never calls `idle()`:** does Foreman ask again after another 20 minutes, or only once per idle stretch? To decide.
- **A CEO shell in the container** (`podman exec`) is a process too, so it keeps the agent awake.
- **The Direct engine** has no container to inspect, so agents there never sleep.
- **A lead waiting on a teammate's reply** sleeps too, and the reply wakes it.
- **A pane the CEO is watching** turns into the asleep view; the Line or Start wakes it.

- [ ] the setting (off, or minutes), in Settings and `settings.toml`
- [ ] the process check in `foreman-agent`, with each adapter's helper list
- [ ] decide: the silent process check alone, or the ping and `idle()` (through Messaging's delivery, draft countdown included)
- [ ] `Sleep(id)` in the instance manager; activity cancels a pending sleep
- [ ] tests: no ping with a stray process, with the setting off, or with messages waiting; `idle()` sleeps and a message wakes; activity after the ping keeps it awake
