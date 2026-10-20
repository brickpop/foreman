---
type: task
status: done
priority: high
tags: [feature]
---

# Stuck agents and the log cap

**Why:** both are specced and their settings (`stuck_after_minutes`, `log_cap_mb`) are validated in `settings.toml`, but nothing reads them: no stuck agent is ever flagged, and session logs grow without a cap. Found by [Settings by topic](./settings-by-topic.md), which left their rows out until they work.

## Decided (the CEO, 2026-10-20)

- **Stuck = mid-turn and silent:** an agent whose status is `running` and from which **no hook event at all** has come for `stuck_after_minutes` (default 20). Hooks fire on every prompt, every tool call (Claude Code: before and after each; Pi: at each start) and every turn end, so a working agent sends one every few seconds to a minute. One rule for every harness, long tools included (Pi can't tell when a tool ends): a genuinely 25-minute test run gets flagged, and the CEO dismisses it. Terminal output doesn't count: Claude Code's spinner redraws while it thinks, so a hung turn never goes quiet.
- **Flagged, it raises a card and the CEO decides:** a blocked request "<Name> may be stuck: no activity for <N> min", with Restart and Open session. The agent keeps running. Any hook event from it resolves the card by itself. (Every other blocked request means the agent already stopped; this one doesn't.)
- **Log cap per agent, long sessions split:** each agent's log folder (`logs/<agent>@<project>/`) stays under `log_cap_mb`, the oldest session files deleted first. A session that alone passes the cap goes on in a new part file, so its oldest part can go too. Nothing reads old logs back (the browser terminal restores from a screen snapshot), so deleting them breaks nothing.

**Done when:** both work, live on a settings change; Settings → Agents → Running agents shows their rows; tests cover the stuck rule (silence, any event resetting it, idle and awaiting agents never flagged, the card resolving itself) and the cap (oldest first, the current file split, never deleting the file being written).

- [x] stuck detection and its request
- [x] the log cap
- [x] the Settings rows
- [x] specs: Agent runtime, Requests, Instance, Persistence, Global settings, Settings

**How it came out:** the Instance manager keeps when each live agent was last heard (any hook, or its run starting) and `CheckStuck` runs once a minute from `cmd/foreman`; the card is a `blocked` request from `foreman` with `Restart`, which the UI sends to the instance's restart route (answering requests isn't built). Its card resolves on a hook, its run ending, or a Foreman restart (`Restore`). The log keeps a session's parts and reads its tail across them: the browser terminal still replays the log's last 1 MB until [Terminal fidelity](../terminal-fidelity.md). `instances.last_output_at`, never written, was dropped (migration 003). Settings → Agents → Running agents has both rows, as selects.
