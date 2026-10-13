---
type: spec
status: planned
tags: [architecture, agents, sessions, acp]
---

# Harness adapters

A **harness** is the CLI program that runs an agent: Claude Code, Codex CLI, Pi, Hermes Agent, or a custom command ([Agent soul](../models/agent-soul.md) `harness` / `command`). Foreman stays harness-agnostic: any CLI coding agent that runs in a terminal can be a team member, as long as an adapter describes how to drive it.

## Integration contract

A harness can be a team member if:
1. Foreman can start it with a working directory, environment and an initial prompt or system prompt file.
2. It can call the ACP tools, exposed as an MCP server ([ACP](./acp.md)).
3. It accepts stdin text while running (CEO input, injected ACP lines).

Foreman never parses harness internals. It reads the output stream and talks to the agent through ACP and stdin.

## Adapter config

One adapter per harness kind. Each declares:

| Field | Meaning |
|---|---|
| `command`, `args` | How to start it. The command comes from [workspace settings](../models/workspace-settings.md) (`claude`, `codex`, `pi`, `hermes`); `custom` uses the soul's `command` |
| system prompt | How the [session prompt](./session-prompt.md) is passed: a flag with a file (e.g. Claude Code `--append-system-prompt` / a system prompt file), a config file written into the sandbox, or the first stdin message |
| MCP registration | How the ACP server is registered: an MCP config file or flag (e.g. Claude Code `--mcp-config`, Codex `mcp_servers` config), pointing at Foreman's ACP listener with the instance token |
| idle / waiting detection | Which signals mean "idle at prompt" and "waiting for input" (below) |
| resume | Whether and how a previous harness session can be resumed ([Agent runtime](./agent-runtime.md#open-questions)) |
| auth | How credentials are passed into the container ([Session runner](./session-runner.md#open-questions)) |
| detection | How to read its version (`<cmd> --version`), shown in [Settings](../features/settings.md) as "found · version" or "not set" |
| rate-limit pattern | How a rate-limit exit or message is recognised, to raise a blocked request with the right options |

Adapter configs are built into Foreman for the known harnesses. `custom` gets a minimal adapter: command, stdin prompt, silence-based idle detection.

## Idle and waiting detection

Status (`running` / `idle` / `awaiting` / `done`) must not depend on scraping a TUI where it can be avoided. **Recommendation:** harness-native hooks first, heuristics as fallback.

- **Hooks:** Claude Code hooks (e.g. `Stop` when a turn ends, `Notification` when it waits for input or permission) configured to call back into Foreman (a tiny command or an HTTP call to the ACP listener with the instance token). Codex has a `notify` program hook. Pi and Hermes to be confirmed.
- **ACP activity:** an open `escalate(approval)` means `awaiting` regardless of output; a `handoff` with nothing assigned means `done`.
- **Fallback:** output silence for a few seconds after a turn plus a per-harness prompt pattern means `idle`.

Idle detection matters beyond the status dot: queued ACP messages and soul updates are delivered "at the next idle prompt" ([ACP](./acp.md#delivery)).

## Open questions

1. **Adapter mechanics per harness** (PRD §16.1): exact mechanism for system-prompt injection and MCP/ACP registration. Claude Code and Codex support MCP; Pi and Hermes need confirmation. Spike per harness; record the result in this entry.
2. **Idle and "waiting for input" detection per harness** (PRD §16.2): prompt pattern, process state or harness events? Recommendation above (hooks first, silence + prompt pattern fallback); spike per harness to confirm what each offers.
3. **Custom harness contract.** What must a custom command support to be usable (MCP config format, prompt passing)? Recommendation: document a minimal contract (reads an MCP config path from an env var, takes the prompt on stdin) and let `custom` fall back to silence detection.
4. **Typing into a busy TUI.** Writing text to a TUI's stdin while it is mid-turn may interleave with its own input handling. Is queueing until idle enough for every harness, and how is "Enter" sent reliably (bracketed paste vs raw `\r`)? Spike.
5. **Terms of use.** Driving a subscription-authenticated interactive harness from an orchestrator may be restricted by the provider's terms (the original PTY proof of concept relied on it). Verify per provider before relying on subscriptions rather than API keys.
