---
type: task
status: done
priority: high
tags: [chore]
epic: terminal-multiplexer
---

# Restructure the code along the building blocks

**Why:** the code from the first steps predates [Building blocks](../../product/architecture/building-blocks.md): `supervisor` mixes the Terminal block with a start-by-id map, and `runner/pty` is named after a mechanism, not where it runs. Everything in this epic builds on the block packages, so they come first.

**Done when:** the packages match the blocks, behaviour is unchanged, and every existing test passes (plus `just race` on the CEO's Mac).

- [x] `runner/pty` → `runner/direct`
- [x] `supervisor` → `terminal`: one process, its `Log`, its views (`Attach`, `Input`, `Resize`, `Done`), no instance map; the `Screen` is wired in by [Terminal fidelity](../terminal-fidelity.md)
- [x] the start-by-instance map moves to `cmd/foreman` scaffolding until the Instance manager exists (it already is debt: `--demo-session`, [technical debt](../../technical-debt.md))
- [x] update [Tech stack → Repo layout](../../product/architecture/tech-stack.md#repo-layout) and the code map in [Building blocks](../../product/architecture/building-blocks.md#code-today-against-these-blocks)
- [x] `just race` on the CEO's Mac (clean after fixing a macOS-only test hang in `direct` and a data race in the vt emulator's Close)
