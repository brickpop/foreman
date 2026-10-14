---
type: task
status: blocked
priority: high
tags: [feature]
---

# Claude Code harness adapter and the login container

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** the first harness ([Harness adapters](../product/architecture/harness-adapters.md)), and the CEO's one login shared by every Claude instance ([Session runner → Credentials](../product/architecture/session-runner.md#credentials)).

Blocked by [Experiment B](./prove-b-shared-login.md) and [Experiment E](./prove-e-prompt-and-mcp-config.md).

**Done when:** `harness/claudecode` implements `Launch` (the session prompt as `CLAUDE.md` in the instance's private config dir, the shared login file, permission mode, git identity), and Settings → Harnesses → Connect runs the login container in a dialog; the fake harness implements the same interface for tests.

- [ ] the `Harness` interface and the fake harness adapter
- [ ] session prompt composer ([Session prompt](../product/architecture/session-prompt.md))
- [ ] private config dirs, shared login folder, `~/.gitconfig` (ro, if present)
- [ ] login container + [Settings → Harnesses](../product/features/settings.md#harnesses)
