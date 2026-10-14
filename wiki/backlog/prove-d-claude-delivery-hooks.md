---
type: task
status: todo
priority: high
tags: [spike]
epic: prove
---

# Experiment D: deliver into Claude Code, and know when it is idle or waiting

**Question:** can Foreman (1) type a message into Claude Code's interface reliably, including while it is busy, and (2) tell from Claude Code's hooks when it is busy, idle at its prompt after finishing, or waiting for the user (a question or a permission dialog)? And how long can a tool call stay open (for `escalate(approval)`) before Claude Code times it out? This is the core of messaging. See [Harness adapters](../product/architecture/harness-adapters.md), [Protocol](../product/architecture/protocol.md), [Building blocks → What is proven](../product/architecture/building-blocks.md#what-is-proven-and-what-isnt).

Claude writes the recipe; the CEO runs it on macOS and Fedora.

**Procedure** (`just prove-d`, Claude Code in a container from experiment B's image, driven through the terminal from experiment A or the Direct engine):
1. Configure hooks (`Stop`, `Notification`, `UserPromptSubmit`, `PreToolUse`) to run a script that appends the event and its JSON payload to a mounted file.
2. Delivery while idle: type a prompt (as raw input, then as a bracketed paste), then Enter; expect it to be submitted exactly once, unchanged.
3. Delivery while busy: start a long task, type a second message mid-turn; record what Claude Code does with it (queued, interleaved, lost).
4. Multi-line message with special characters; record how it arrives.
5. Make it ask the user something in plain text, and trigger a permission dialog; record which hook events fire and whether their payload tells "waiting for the user" apart from "finished".
6. Register a test MCP tool that sleeps; find how long a call may block before Claude Code gives up.
7. Shared login in interactive sessions (left over from [experiment B](./prove-b-shared-login.md), which proved the shared login file in print mode): run a few interactive sessions on the shared login file across a token expiry (forced, as B's race does), so the refresh happens mid-session; every session keeps working and the shared file holds the refreshed token.

**PASS:** a delivery to an idle instance is always submitted once and intact; the hooks distinguish busy, idle and waiting-for-user; a tool call can stay open long enough for an approval (or keep-alives extend it). **FAIL:** any of these doesn't hold.

**On FAIL:** [Protocol](../product/architecture/protocol.md) and [Harness adapters](../product/architecture/harness-adapters.md) change: e.g. deliveries only at idle with a submit check, an output heuristic for waiting, or approvals that return at once and resume by delivery. The CEO reviews the change before epic 2.
