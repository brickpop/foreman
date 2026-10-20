---
type: task
status: done
priority: high
tags: [spike]
epic: prove
---

# Experiment D: deliver into Claude Code, and know when it is idle or waiting

**Question:** can Foreman (1) type a message into Claude Code's interface reliably, including while it is busy, and (2) tell from Claude Code's hooks when it is busy, idle at its prompt after finishing, or waiting for the user (a question or a permission dialog)? And how long can a tool call stay open (for `escalate(approval)`) before Claude Code times it out? This is the core of messaging. See [Harness adapters](../../product/architecture/harness-adapters.md), [Protocol](../../product/architecture/protocol.md), [Building blocks → What is proven](../../product/architecture/building-blocks.md#what-is-proven-and-what-isnt).

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

**On FAIL:** [Protocol](../../product/architecture/protocol.md) and [Harness adapters](../../product/architecture/harness-adapters.md) change: e.g. deliveries only at idle with a submit check, an output heuristic for waiting, or approvals that return at once and resume by delivery. The CEO reviews the change before epic 2.

## Progress

- **Driver written (2026-10-14), on Foreman's real code**: `just prove-d [seconds]` starts an interactive Claude Code through `runner/podman` on the image from the real image builder, feeds its output to `internal/screen` (which also answers its terminal queries), and receives its hooks (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Notification`, `Stop`) through the agent stub on a host listener. Onboarding and folder trust are seeded in the private `.claude.json` (the login's account record, plus `hasCompletedOnboarding` and the folder's `hasTrustDialogAccepted`). Steps 1 to 7 as above, with step 6's hold time a parameter (default 330 s). Every hook event and a screen dump per step go to `.dev/prove/d/<date-time>/report.txt`, one folder per run.
- **Runs 1 and 2 (2026-10-14, macOS), findings so far:**
  - Hooks reach the host from inside the container; seeding `.claude.json` (onboarding, folder trust) reaches the prompt with no dialog.
  - A short message, typed or pasted, is submitted once and unchanged (one exception, under study: in run 2 a typed message was never submitted). A message typed mid-turn is queued and answered in the same turn.
  - **A multi-line paste arrives wrapped**: Claude Code submits it as `\n\n<pasted_content id="…">\n<text>\n</pasted_content id="…">\n`, with tabs turned into four spaces; everything else intact. A short single-line paste is not wrapped. Claude treats pasted content as not necessarily the user's own words, so a delivery must not depend on a multi-line paste: the delivery form is to decide with the [Messaging](../epics/messaging-handoffs.md) design.
  - **Waiting for the CEO is visible**: asked to ask the user something, Claude Code uses its question dialog (the `AskUserQuestion` tool: `PreToolUse` with that `tool_name`), then fires `Notification` with `notification_type: "permission_prompt"` about 9 s later.
  - Step 7 (run 3): with the shared token marked expired mid-session, the next turn refreshed it and the refreshed token landed in the shared file: an interactive session re-reads the shared login file. (Run 2's failure here came from a session left in a muddled state by an earlier step.)
  - The CEO's decision (2026-10-14): agents run **with permission checks off** (`--dangerously-skip-permissions`, the one-time confirmation seeded), as [Harness adapters](../../product/architecture/harness-adapters.md) already says; the container is the boundary, and the agent works in one go. Step 5b now checks that a change runs straight through.
- **Run 3 (2026-10-14, macOS, permission checks off): the answers.**
  - Bypass mode's one-time warning is skipped by `skipDangerousModePermissionPrompt: true` in the instance's `settings.json` (the `.claude.json` key alone doesn't do it).
  - Delivery: a short message typed or pasted at an idle prompt is submitted once, unchanged; typed mid-turn, it is queued and answered in that turn. A **multi-line paste is wrapped** as pasted content (stable across runs): the Claude Code adapter's `Deliver` must not paste multi-line messages. Options, in order: type them with Claude Code's new-line key between lines (to test in the adapter's integration tests), or deliver a one-line notice and let the agent read the full message through a team tool.
  - Waiting for the CEO: a question goes through `AskUserQuestion` (`PreToolUse`), then `Notification` (`notification_type: "permission_prompt"`) about 9 s later; with permission checks off, a change runs with no dialog and no signal.
  - **A long tool call is never cut off: after 120 s Claude Code moves it to the background** (the result says so), ends the turn with a `Stop` listing it under `background_tasks`, and keeps taking messages. An approval held open in a tool call would survive, but the cleaner design is still an `escalate(approval)` that returns at once, the answer arriving as a delivery: decided with [Messaging and hand-offs](../epics/messaging-handoffs.md).
  - The `Stop` payload carries `last_assistant_message`: the turn's last words, for markers and "done" summaries.
  - Shared login: refreshed mid-session into the shared file (step 7).

**Verdict: PASS for Claude Code on macOS**, with one design consequence: multi-line deliveries are not pasted. Fedora was not run (macOS is the CEO's machine). The checks move into the Claude Code adapter's integration tests ([Claude Code harness](./claude-harness-and-login.md)), after which `cmd/prove` is deleted.
