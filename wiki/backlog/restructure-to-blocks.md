---
type: task
status: todo
priority: high
tags: [chore]
epic: terminal-multiplexer
---

# Restructure the code along the building blocks

**Why:** the code from the first steps predates [Building blocks](../product/architecture/building-blocks.md): `supervisor` mixes the Terminal block with a start-by-id map, and `runner/pty` is named after a mechanism, not where it runs. Everything in this epic builds on the block packages, so they come first.

**Done when:** the packages match the blocks, behaviour is unchanged, and every existing test passes (plus `just race` on the CEO's Mac).

- [ ] `runner/pty` → `runner/direct`
- [ ] `supervisor` → `terminal`: one process, its `Screen`, its `Log`, its views (`Attach`, `Input`, `Resize`, `Done`), no instance map
- [ ] the start-by-instance map moves to `cmd/foreman` scaffolding until the Instance manager exists (it already is debt: `--demo-session`, [technical debt](../technical-debt.md))
- [ ] update [Tech stack → Repo layout](../product/architecture/tech-stack.md#repo-layout) and the code map in [Building blocks](../product/architecture/building-blocks.md#code-today-against-these-blocks)
