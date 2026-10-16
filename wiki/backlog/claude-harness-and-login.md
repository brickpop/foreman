---
type: task
status: done
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Claude Code harness adapter and the login container

**Why:** the first harness ([Harness adapters](../product/architecture/harness-adapters.md)), and the CEO's one login shared by every Claude instance ([Session runner → Credentials](../product/architecture/session-runner.md#credentials)).

Unblocked: [Experiment B](./prove-b-shared-login.md) and [Experiment E](./prove-e-prompt-and-mcp-config.md) passed on macOS.

**Done when:** `harness/claudecode` implements `Launch` (the session prompt as `CLAUDE.md` in the instance's private config dir, the shared login file, permission mode, git identity), and Settings → Harnesses → Connect runs the login container in a dialog; the fake harness implements the same interface for tests.

- [x] the `harness.Adapter` interface (`Launch` for now: `Deliver`, `Interpret` and `Diagnose` come with the agent link, in [Messaging and hand-offs](./epics/messaging-handoffs.md)) and the fake harness adapter
- [x] session prompt composer ([Session prompt](../product/architecture/session-prompt.md)), with an epic 1 variant: without the agent link it lists no team tools, and says the agent works alone and leaves its work uncommitted for the CEO
- [x] Claude Code `Launch`: private config dir in the sandbox (`.harness/claude-code`), `CLAUDE.md`, the shared login file, `.claude.json` seeded (onboarding, trust), `skipDangerousModePermissionPrompt`, `tui: "default"` (no fullscreen-renderer dialog, no mouse capture), `--dangerously-skip-permissions`, `--continue` on a restart, `~/.gitconfig` (ro, if present)
- [x] the login container's server side: the adapter's `Login()` and `Connected()`; `GET /api/harnesses`, `POST /api/harnesses/:id/connect` (the login runs as terminal `connect:<harness>`, reused while it runs); its dialog is the [Settings screen](./ui-settings-notifications.md)'s. `cmd/prove`'s login stays while experiment D's driver needs it ([Messaging and hand-offs](./epics/messaging-handoffs.md))
