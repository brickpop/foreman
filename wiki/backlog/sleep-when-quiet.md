---
type: task
status: todo
priority: high
tags: [feature]
epic: director
---

# Sleep after a quiet spell

**Why:** the CEO, 2026-11-05. An idle agent holds a container, a session slot and memory while waiting for nothing. Today it only gives them up when another instance waits for its slot ([Agent runtime → Idle agents make room](../product/architecture/agent-runtime.md#idle-agents-make-room)). Wanted: any agent quiet for a while goes to sleep, and the next message wakes it with its conversation (`--continue`). Needed for the [Director](./epics/director.md), which waits for leads' reports for hours; useful for every agent.

**Decided:**

- A global setting **`sleep_after_minutes`**, default **7**, > 0, in Settings → Agents; read at each check, so a change applies at once.
- **Quiet** means exactly what it means today for making room: `idle` (never `running` or `awaiting`: an open dialog would be lost), no input since (the CEO's keystrokes or a delivery restart the clock), no unsent draft of the CEO's in its prompt, and **nothing else running** in its session (a dev server, a watcher, a build, a shell the CEO opened: the existing process check). The CEO typing in an agent's terminal is activity: it resets the clock, it doesn't need a wake.
- The sleep itself is today's (terminal closed to input in the same step as the last check, the container stopped, never `/exit`), and so is the wake (a delivery or the CEO's Start, continuing the conversation).
- **Making room stays:** when an instance waits for a slot, an agent quiet for a minute still sleeps sooner, as today.
- Applies to every instance, the Director included.
- **Known edge, accepted (the CEO, 2026-11-08):** a harness timer (Claude Code's `ScheduleWakeup`, `/loop`, session cron) has no process, so the check can't see it, and it is lost when the agent sleeps. Not worked around. (If it bites: Foreman sees those tool calls in its hooks and could keep the agent awake or wake it on time.)

**Side effects to expect:** each wake costs a container start and the harness's start (seconds); a restarted agent picks up the current team file, prompt and tool list (so team edits and new tools reach agents sooner: it eases [An agent keeps an old tool schema](./tool-schema-after-restart.md)); more starts mean more restores to test.

**Steps:**

- [ ] the setting (`home.Settings`, its default and bounds), its row in Settings → Agents
- [ ] the instance manager's check: once a minute, every quiet agent past the setting sleeps (the make-room path reused, not a second one)
- [ ] specs: [Agent runtime](../product/architecture/agent-runtime.md) (idle agents: the timed sleep beside making room, the transition table, the timers edge), [Global settings](../product/models/global-settings.md), [Settings](../product/features/settings.md)
- [ ] tests: sleeps after the setting and not before; a keystroke, a delivery or a draft restarts the clock; a background process keeps it awake; `awaiting` never sleeps; a changed setting applies at the next check; making room still works
