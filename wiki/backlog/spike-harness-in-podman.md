---
type: task
status: todo
priority: high
tags: [spike]
---

# Spike: run each harness in rootless Podman

Prove that Claude Code and Codex CLI (then Pi and Hermes) run interactively in a rootless Podman container on Linux and on macOS (podman machine), with only a sandbox mounted. Feeds [Session runner](../product/architecture/session-runner.md) and [Harness adapters](../product/architecture/harness-adapters.md).

- [ ] Authentication inside the container (Claude Code `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`, Codex API key or login file); one credential shared vs one per instance
- [ ] Injecting the system prompt and registering an MCP server per harness
- [ ] Idle and waiting-for-input detection through native hooks (Claude Code hooks, Codex `notify`)
- [ ] Typing into a busy TUI: queued input and a reliable Enter
- [ ] `--userns=keep-id` file ownership in the sandbox; bind-mount speed on macOS
- [ ] Write the findings into the specs, then delete this card
