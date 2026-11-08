---
type: task
status: todo
priority: low
tags: [feature]
epic: install-first-run
---

# Free memory before the machine runs out

**Why:** the CEO, 2026-11-08. Agent containers have no memory limit. When several agents (or a build one of them runs) need more than the Podman machine has, the VM swaps or its out-of-memory killer picks a process anywhere in it: agents hang. [Idle agents make room](../product/architecture/agent-runtime.md#idle-agents-make-room) frees an idle agent only when another waits for a **slot**, never for memory. A 20-minute idle timer was considered instead and dropped: memory is what it would protect, and this protects it directly. The timer existed once and was replaced ([Idle sleep](./archive/idle-sleep.md)).

**Done when:** a start waits while memory is low, idle agents sleep under memory pressure, a runaway container dies alone with "out of memory" as its reason, and [Agent runtime](../product/architecture/agent-runtime.md) (Idle agents make room, Concurrency, the status table) and [Session runner](../product/architecture/session-runner.md) describe it.

## 1. Probe first (on the CEO's machine)

Nothing below is built until these are measured:

- [ ] does `host.memFree` from `GET /libpod/info` mean free or *available* memory? Compare with `MemAvailable` in the VM's `/proc/meminfo` (free memory excludes reclaimable cache, so it reads too low)
- [ ] what an agent uses, from `GET /libpod/containers/stats?stream=false`: Claude Code and Codex, each idle, mid-turn, and running a typical build or test
- [ ] a container started with `--memory` that goes over it: only its process dies, Foreman sees an exit, nothing else hangs; how Podman's inspect flags it (OOM-killed)
- [ ] the prompt cache across a restart: the first turn's cache reads after `claude --continue` / `codex resume --last`, with and without a commit in between (Claude Code logs `cache_read_input_tokens` in its transcript, Codex `cached_input_tokens`). Claude Code and Codex rebuild their own context (date, git status, environment) on a new process; whether that sits before the conversation and misses the cache is unknown. Foreman's own prompt is deterministic already (`prompt.Compose`, hashed). Only matters if an idle timer comes back

The results pick the marks and the limit below.

## 2. Memory gate and pressure sleep

- **`Memory(ctx)` on the runner**, `{total, available}`: Podman from the machine (the source the probe settles); Direct from `/proc/meminfo` on Linux, "unknown" on a Mac (the gate is off: a technical-debt item)
- **Gate:** a start or a wake stays `queued` while available memory is under the start mark; the Activity panel says "waiting for memory". Make-room then sleeps an idle agent as it does for a slot: no new mechanism
- **Pressure:** in make-room's once-a-minute look, under the lower mark it sleeps idle agents one at a time, the largest (by `stats`) first, until memory is back over the mark. Same guards as today: idle a minute, no input since, no unsent draft, nothing running but `foreman-agent`
- **Everyone busy and still under the mark:** one card for the CEO ("out of memory: N agents working, X GB used of Y") that resolves itself. Nothing is killed
- **Fixed marks, no setting** (the CEO, 2026-11-08: a setting only once there's clarity). Starting guesses: 1.5 GB to start, 750 MB to sleep; the probe sets them

## 3. A memory limit per container

- every agent container starts with `--memory` at a share of the machine (half, unless the probe says otherwise), so a runaway build dies in its own container and the existing crash path restarts it once, instead of freezing the VM
- the stop reason says "out of memory", from the container's OOM-killed flag, not an unexplained crash

## Tests

- [ ] a start waits under the mark and goes once memory frees
- [ ] pressure sleeps the largest idle agent, stops once over the mark; never a `running` / `awaiting` or non-quiet one
- [ ] "unknown" memory turns the gate off
- [ ] the out-of-memory card raises once and resolves itself
- [ ] `--memory` reaches Podman; an OOM-killed exit gets its reason; integration: one container over its limit, the others unaffected

## Still open

- the marks and the limit's share, from the probe
- whether an idle timer is still wanted after this (the cache step of the probe informs it); a setting for it only then
