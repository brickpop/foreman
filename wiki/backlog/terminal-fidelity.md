---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Terminal fidelity: snapshots of full-screen programs

**Why:** agents' harnesses are full-screen programs; reloads and late viewers must see them exactly ([Terminal state](../product/architecture/terminal-state.md)). Most of it is built and parked in `internal/screen` (round-trip tests, every-byte-offset test, query answers).

Unblocked: [Restructure along the building blocks](./restructure-to-blocks.md) is done.

**Done when:** the Terminal uses `Screen` snapshots on attach, the browser never answers terminal queries (xterm.js handlers overridden), and the round-trip tests pass on synthetic streams and on recordings from the CEO's Mac (`vim`, `htop`, `less`, Claude Code).

- [ ] wire `internal/screen` into the Terminal (snapshot, query answers)
- [ ] the pending-wrap case is accepted for now and tracked apart, not blocking this card: [Snapshots can't restore a pending line wrap](./snapshot-pending-wrap.md)
- [ ] xterm.js: never answer queries
- [ ] fake harness full-screen mode; recordings; the manual checklist
