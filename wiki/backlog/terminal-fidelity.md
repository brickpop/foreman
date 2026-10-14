---
type: task
status: blocked
priority: low
tags: [feature]
---

# Terminal fidelity: snapshots of full-screen programs

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** agents' harnesses are full-screen programs; reloads and late viewers must see them exactly ([Terminal state](../product/architecture/terminal-state.md)). Most of it is built and parked in `internal/screen` (round-trip tests, every-byte-offset test, query answers).

Blocked by [Restructure along the building blocks](./restructure-to-blocks.md).

**Done when:** the Terminal uses `Screen` snapshots on attach, the browser never answers terminal queries (xterm.js handlers overridden), the pending-wrap edge case is resolved, and the round-trip tests pass on synthetic streams and on recordings from the CEO's Mac (`vim`, `htop`, `less`, Claude Code).

- [ ] wire `internal/screen` into the Terminal (snapshot, query answers)
- [ ] the pending-wrap case: the emulator keeps it private; options to take to the CEO (upstream accessors, reading the pinned library's state, or accepting it)
- [ ] xterm.js: never answer queries
- [ ] fake harness full-screen mode; recordings; the manual checklist
