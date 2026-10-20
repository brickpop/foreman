---
type: spec
status: partial
tags: [architecture, agents, sessions, team-tools]
---

# Harness adapters

A **harness** is the CLI program that runs an agent: Claude Code, Codex CLI, Pi, Hermes Agent, or a custom command. A [profile](./session-runner.md#credentials) names one, with its credentials, and a project's agents run on its profile's. Foreman stays harness-agnostic: any CLI coding agent that runs in a terminal can be a team member, as long as an adapter describes how to drive it.

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

- **Hooks:** the harness's own hooks, configured by the adapter to run `foreman-agent hook <event>`, which relays them through the [agent link](./building-blocks.md#4-agent-link-agentlink--foreman-agent): Claude Code `Stop` when a turn ends, `Notification` when it waits for the user. Codex has a `notify` program hook (later). Pi has no shell hooks: an extension relays its events ([Pi](#pi)). Hermes to be confirmed.
- **Claude Code's events, as built** (from [experiment D](../../backlog/archive/prove-d-claude-delivery-hooks.md)): `SessionStart` and `Stop` → idle; `UserPromptSubmit`, `PostToolUse` and any other `PreToolUse` → running; `PreToolUse` of `AskUserQuestion`, and `Notification` with `notification_type: "permission_prompt"` → awaiting; a `Notification` with `idle_prompt` → idle; anything else changes nothing. Hooks only move an instance between `running`, `idle` and `awaiting`: they never wake, start or revive one.
- **Idle is not waiting.** `Interpret` must tell apart a harness that finished and sits at its prompt (`idle`: nothing is raised) from one that waits **for the user** (`waiting`: a permission dialog, a choice, or a question it asked before stopping), which raises a request on the Dashboard ([Protocol → Waiting for the CEO](./protocol.md#waiting-for-the-ceo)). For Claude Code, whether its `Notification` hook payload says which case it is (and carries the question text and choices) is verified in the [Prove epic](../../backlog/epics/prove.md); if it can't, the fallback is to treat a `Notification` that follows a `Stop` within a short delay as idle.
- **Team-message activity:** an open `escalate(approval)` means `awaiting` regardless of output; a `handoff` with nothing assigned means `done`.
- **Fallback:** output silence for a few seconds after a turn plus a per-harness prompt pattern means `idle`.

Idle detection matters beyond the status dot: queued team messages and soul updates are delivered "at the next idle prompt" ([Team tools](./team-tools.md#delivery)).

## Activity lines

The same hooks also say **what** an agent is doing, as one line for the CEO on [Project settings](../features/project-settings.md) rows (decided by the CEO, 2026-10-18, rather than the terminal's last line, which for a full-screen harness is its own footer). The adapter's `Describe(event, payload)` returns the line, or `""` when the event says nothing new; the Instance manager keeps the latest per instance in memory, keeps only its first line without control characters, cut at 200 characters, sends it as `instance.doing` and lists it with the instances. A new run starts without one; a hook before the instance runs, or after it stops, is ignored, as for statuses.

Claude Code's, as built: `SessionStart` → `Started`; `UserPromptSubmit` → `Working on “<the message>”`; `PreToolUse` → the tool and what it works on (`Bash: <command>`, `Reading` / `Editing` / `Writing <path>`, relative to the sandbox when inside it, `Searching for <pattern>`, `Fetching <url>`, `Subagent: <description>`, `Asked you a question`, `Team tool: <tool>` for Foreman's, `<server>: <tool>` for another MCP server's, else the tool's name); `Notification` of a `permission_prompt` → `Waiting for a permission`; `Stop` → `Finished its turn`. A tool's result (`PostToolUse`) leaves the line on the tool.

## Pi

Built 2026-10-18 ([Pi as a second harness](../../backlog/pi-harness.md)), probed on Pi 1.3.0 (`@earendil-works/pi-coding-agent`, installed in the agent image after Claude Code).

- **Its profile is a model server, not a login** (the CEO, 2026-10-18): an OpenAI-compatible URL, an optional API key, one model and its sizes, set in a form ([Settings → Profiles](../features/settings.md#profiles)) and saved as Pi's own `models.json` in the profile folder. One provider, one model per profile; fields the CEO adds to the file by hand (`samplingParams`, `thinkingLevelMap`, `reasoning`…) are kept by a save while the model stays the same. Pi's own `/login` (subscriptions, built-in providers) isn't offered.
- **Each instance** gets its own config dir, `<sandbox>/.harness/pi` (`PI_CODING_AGENT_DIR`): a copy of the profile's `models.json` in which a loopback host (`localhost`, `127.0.0.1`, `::1`) becomes the host alias, `host.containers.internal`, when agents run in containers (never on the Direct engine, never another host), and a key is always set (`none` when there is none: Pi offers no model without one); the [session prompt](./session-prompt.md#delivery) as `APPEND_SYSTEM.md`; `mcp.json` with `foreman-agent mcp` at `"exposure": "direct"` (Pi's default exposes MCP tools only to its `codemode` scripts); and the hooks extension.
- **Command:** `pi --no-approve --tui-mode regular --provider <p> --model <id> [--extension <hooks>] [--continue] [-- <opener>]`, with `PI_SKIP_VERSION_CHECK=1` and `PI_TELEMETRY=0`. `--no-approve` declines project trust: no dialog at startup, and a repo's own `.pi/` (settings, extensions, MCP servers, skills) never loads. `--tui-mode regular` leaves the mouse and the scrollback to the pane's terminal: a selection copies in the browser, and the pane's Copy gets the whole conversation. In Pi's default fullscreen mode it copies a selection itself, into the container, which has no clipboard, so copies were lost (the CEO, 2026-10-19). `--provider` and `--model` take the id exactly (`qwen3:30b`, `qwen/qwen3`). The opener follows `--` so one starting with `-` is still the message; it is taken with `--continue` too. Continuing follows Claude Code's rule: only this deployment's conversation, else fresh.
- **Hooks:** a TypeScript extension the adapter writes (`foreman-hooks.ts`) runs `foreman-agent hook <event>` with the event's JSON on stdin, waiting for each so they arrive in order: `session_start` → idle, `Started`; `before_agent_start` (a run begins; a slash command starts none) → running, `Working on “<the message>”`; `tool_call` → running, the tool (`Bash:`, `Reading` / `Editing` / `Writing <path>`, `Searching for`, `Looking for`, `Listing`, `Running a script`, `Asked you a question`, `Team tool: <tool>`); `agent_settled` (final, after any retry or compaction) → idle, `Finished its turn` or `Stopped`; `ui_prompt_start` → awaiting, `ui_prompt_end` → running.
- **`ask_user`** (the CEO, 2026-10-18): Pi has no tool to ask the user, as Claude Code's `AskUserQuestion` does, so the same extension adds one: `ask_user(question, options?)`, declared to the model only (`model-only`). It opens a dialog in Pi's terminal: a choice of the options plus "Something else (type it)", or a text answer; the model gets "The user answered: …" or "The user dismissed the question without answering." Its tool call reads `Asked you a question`, and the dialog's opening and closing make the agent awaiting, then running, as Claude Code's question does. Probed on Pi 1.3.0.
- **Newline:** Ctrl+J (`\n`), which Pi binds beside Shift+Enter.
- **Model detection:** the form's Detect asks `GET <url>/models` from Foreman itself (so `localhost` is this machine), with the key typed or the one set.
- **Not covered yet:** a model served on the host and reached from Podman on **Linux** (the same open question as the agent port: [First run](../../backlog/first-run.md)); a check of a real model through the agent port end to end (the CEO's, on the Mac).

## Open questions

1. **Adapter mechanics per harness** (PRD §16.1): exact mechanism for system-prompt injection and MCP/team-tools registration. Claude Code and Pi are built ([Pi](#pi)); Codex supports MCP; Hermes needs confirmation. To confirm in the [Prove epic](../../backlog/epics/prove.md); record the result in this entry.
2. **Typing into a busy TUI.** Writing text to a TUI's stdin while it is mid-turn may interleave with its own input handling. Is queueing until idle enough for every harness, and how is "Enter" sent reliably (bracketed paste vs raw `\r`)? Confirm in the [Prove epic](../../backlog/epics/prove.md).
