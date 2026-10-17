---
type: task
status: done
priority: medium
tags: [feature]
epic: install-first-run
---

# Stop the agents' containers when Foreman exits

**Why:** the CEO, 2026-10-16. Today containers outlive Foreman so a restart re-attaches them ([Agent runtime → Restarts](../product/architecture/agent-runtime.md#restarts)). But while Foreman is gone nothing reads their output, answers their team tools or delivers their messages, so they only hold memory and CPU, and may spend tokens on a turn nobody follows. Replaces the "Containers when Foreman stops" idea (a setting).

**Decided (the CEO, 2026-10-16):**
- **On a clean exit** (Ctrl-C, SIGTERM), Foreman stops its own containers: the active deployments' agents, and a login container if one is open.
- **On the next start, those agents are asleep.** A message, a hand-off or the CEO's Start wakes each one, picking up its conversation. Nothing runs until there's work, and an agent stopped mid-turn waits too.

**Design notes:**
- **A crash can't stop anything.** The containers keep running, and the next start re-attaches them as today. Restore stays for that case.
- **Mid-turn work:** an agent stopped mid-turn loses only that turn's unfinished part. Its conversation is saved (Claude Code writes as it goes), and `--continue` resumes it on wake.
- **An exit is not a crash:** stopping marks each agent asleep (the instance manager's stopping flag), never `stopped`, and the restart counter is untouched.
- **The stop is bounded:** all containers stop in parallel, each with the usual grace (SIGTERM, then kill), so a second Ctrl-C isn't needed. A second Ctrl-C during the stop exits at once, leaving the rest to Restore.
- **The Podman machine (macOS) and a Linux Podman service** the CEO started stay up. A service Foreman started ends with it, as today.
- **Messages waiting** for an agent at exit wake it on the next start, through the startup delivery (`Sweep`).

- [x] signal handling in `cmd/foreman` (a graceful HTTP shutdown, then the manager stops every live instance as asleep)
- [x] the specs: agent runtime (restarts), session runner (containers outlive Foreman only after a crash)
- [x] tests: a clean exit leaves no container and every agent asleep, with its conversation continued on wake; messages waiting at exit wake their agent on the next start; a crash (containers still running) still re-attaches

**Built 2026-10-16:** `Manager.Shutdown` (every live instance stopped in parallel and set asleep, queued ones too; deploy, wake, starts and re-attaching refuse meanwhile), and `serveUntil` in `cmd/foreman`, which stops serving, puts the signals back and then shuts down the manager, the login terminals and the agent port. Checked end to end with the fake harness: two idle agents, Ctrl-C, no harness process left, both asleep on the next start.

Found while building it: the Linux Podman service Foreman starts was in Foreman's process group, so the terminal's Ctrl-C would have killed it at once and every stop would have failed. It now has its own group, and `Pdeathsig` still ends it if Foreman dies. The crash case still re-attaches (`TestRestore`).
