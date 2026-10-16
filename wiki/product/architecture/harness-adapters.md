---
type: spec
status: planned
tags: [architecture, agents, sessions, team-tools]
---

# Harness adapters

A **harness** is the CLI program that runs an agent: Claude Code, Codex CLI, Pi, Hermes Agent, or a custom command ([Agent soul](../models/agent-soul.md) `harness` / `command`). Foreman stays harness-agnostic: any CLI coding agent that runs in a terminal can be a team member, as long as an adapter describes how to drive it.

## Session drivers

**Interactive sessions only (the CEO's rule).** Every agent session in the product is the harness's real interactive interface in a terminal. The product never runs a harness in a non-interactive or automation mode (Claude Code's `claude -p` / `--print`, or any equivalent "print and exit" mode): an agent is always a live session the CEO can watch and type into. (The Prove experiments may use print mode as a measuring tool; that is not product behaviour.)

**Decided:** how Foreman drives a harness session is itself swappable, through a **session driver** per harness, with two implementations:

| Driver | How Foreman talks to the harness | What the CEO sees | Status |
|---|---|---|---|
| **PTY** | The harness's own TUI in a TTY ([Session runner](./session-runner.md)). Input is bytes on stdin; status comes from harness hooks and output heuristics (below) | The real terminal, live in the browser ([Sessions](../features/sessions.md)) | **v1**, every harness |
| **Agent Client Protocol** ([agentclientprotocol.com](https://agentclientprotocol.com), Zed's open standard; adapters exist for Claude Code and Codex, and a Go SDK) | JSON-RPC over the container's stdio: prompts in; streamed messages, tool calls and plan updates out; permission requests that map naturally onto approval [requests](../features/requests.md); the client can hand the agent Foreman's MCP tools when a session opens | A structured transcript instead of a raw terminal | Later, per harness, where it proves better |

Both drivers expose the same operations to the Instance manager (start with a prompt, send input, observe status, stop), so [Agent runtime](./agent-runtime.md) doesn't know which one runs. The agents' team tools are an MCP server in both cases ([Team tools](./team-tools.md)). Why PTY first: it's what the PRD and the prototype show (a terminal multiplexer), it works for any harness that runs in a terminal, and it keeps subscription logins usable. The Agent Client Protocol adapter for Claude Code runs on the Agent SDK, which means API billing.

### Integration contract (PTY driver)

A harness can be a team member if:
1. Foreman can start it with a working directory, environment and a standing-instructions file ([Session prompt → Delivery](./session-prompt.md#delivery)).
2. It can call the team tools, exposed as an MCP server ([Team tools](./team-tools.md)).
3. It accepts input while running: the CEO's keystrokes, and deliveries: one plain line, then Enter, typed when it is idle ([Building blocks → Messaging](./building-blocks.md#6-messaging-messaging)).

Foreman never parses harness internals. It reads the output stream and talks to the agent through team tools and stdin.

## Adapter config

One adapter per harness kind. Each declares:

| Field | Meaning |
|---|---|
| `command`, `args` | How to start it. The harness's own command (`claude`, `codex`, `pi`, `hermes`), installed in the agent image Foreman builds, so there is no setting for it; `custom` uses the soul's `command` |
| system prompt | Which user-level instruction file the harness reads from its config directory (Claude Code `CLAUDE.md`, Codex `AGENTS.md`), where the [session prompt](./session-prompt.md#delivery) is written. Custom harnesses use a flag with a file path or the first stdin message |
| MCP registration | How the team-tools server is registered: an MCP config file or flag (e.g. Claude Code `--mcp-config`, Codex `mcp_servers` config) declaring the stdio server `foreman-agent mcp` ([Team tools → Transport](./team-tools.md#transport)) |
| newline keystroke | What starts a new line in its input without sending it, typed for the CEO's Shift+Enter in the terminal and for each line break of the line input ([Sessions](../features/sessions.md)). Claude Code: `\` then Enter (its documented quick escape; to confirm on the CEO's Mac against Option+Enter and Ctrl+J). Sent to the UI with the harness status (`GET /api/harnesses`) |
| idle / waiting detection | Which signals mean "idle at prompt" and "waiting for input" (below) |
| opener | How the session starts with a first message and stays interactive: the lead's kickoff ([Agent runtime → Lazy starts](./agent-runtime.md#lazy-starts)). Given only to a session that starts fresh: a restart that continues a conversation doesn't get it again, and one with nothing to continue does ([Agent runtime → Restarts](./agent-runtime.md#restarts)). Claude Code: the message as its prompt argument, after the flags (`claude [options] [prompt]` "starts an interactive session by default"; only `-p` makes it one-shot, which Foreman never uses) |
| resume | Whether and how a previous harness session can be resumed ([Agent runtime](./agent-runtime.md#restarts)) |
| auth | How credentials are passed into the container ([Session runner → Credentials](./session-runner.md#credentials)): which config directory variable the harness honours, where its credentials file lives, which API-key variables it reads |
| detection | How to read its version (`<cmd> --version`, run inside the harness image), shown in [Settings](../features/settings.md) as "found · version" or "not set" |
| permission mode | Inside the container the harness runs without its own permission prompts (e.g. Claude Code `--dangerously-skip-permissions`, as `pclaude` does): the container is the boundary, and anything that needs the CEO goes through `escalate` and the project gates |
| rate-limit pattern | How a rate-limit exit or message is recognised, to raise a blocked request with the right options |

Adapter configs are built into Foreman for the known harnesses. `custom` gets a minimal adapter, and a custom command must meet a minimal documented contract: it reads its MCP config from the path in `FOREMAN_MCP_CONFIG` (to reach the team tools), takes its prompt on stdin, and runs in a terminal. Its idle detection is silence-based.

## Idle and waiting detection

Status (`running` / `idle` / `awaiting` / `done`) must not depend on scraping a TUI where it can be avoided: **harness-native hooks first, heuristics as fallback**. What each harness actually offers is confirmed in the [Prove epic](../../backlog/epics/prove.md).

- **Hooks:** the harness's own hooks, configured by the adapter to run `foreman-agent hook <event>`, which relays them through the [agent link](./building-blocks.md#4-agent-link-agentlink--foreman-agent): Claude Code `Stop` when a turn ends, `Notification` when it waits for the user. Codex has a `notify` program hook (later). Pi and Hermes to be confirmed.
- **Claude Code's events, as built** (from [experiment D](../../backlog/prove-d-claude-delivery-hooks.md)): `SessionStart` and `Stop` → idle; `UserPromptSubmit`, `PostToolUse` and any other `PreToolUse` → running; `PreToolUse` of `AskUserQuestion`, and `Notification` with `notification_type: "permission_prompt"` → awaiting; a `Notification` with `idle_prompt` → idle; anything else changes nothing. Hooks only move an instance between `running`, `idle` and `awaiting`: they never wake, start or revive one.
- **Idle is not waiting.** `Interpret` must tell apart a harness that finished and sits at its prompt (`idle`: nothing is raised) from one that waits **for the user** (`waiting`: a permission dialog, a choice, or a question it asked before stopping), which raises a request on Home ([Protocol → Waiting for the CEO](./protocol.md#waiting-for-the-ceo)). For Claude Code, whether its `Notification` hook payload says which case it is (and carries the question text and choices) is verified in the [Prove epic](../../backlog/epics/prove.md); if it can't, the fallback is to treat a `Notification` that follows a `Stop` within a short delay as idle.
- **Team-message activity:** an open `escalate(approval)` means `awaiting` regardless of output; a `handoff` with nothing assigned means `done`.
- **Fallback:** output silence for a few seconds after a turn plus a per-harness prompt pattern means `idle`.

Idle detection matters beyond the status dot: queued team messages and soul updates are delivered "at the next idle prompt" ([Team tools](./team-tools.md#delivery)).

## Open questions

1. **Adapter mechanics per harness** (PRD §16.1): exact mechanism for system-prompt injection and MCP/team-tools registration. Claude Code and Codex support MCP; Pi and Hermes need confirmation. To confirm in the [Prove epic](../../backlog/epics/prove.md); record the result in this entry.
2. **Typing into a busy TUI.** Writing text to a TUI's stdin while it is mid-turn may interleave with its own input handling. Is queueing until idle enough for every harness, and how is "Enter" sent reliably (bracketed paste vs raw `\r`)? Confirm in the [Prove epic](../../backlog/epics/prove.md).
