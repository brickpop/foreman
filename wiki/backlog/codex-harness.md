---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: install-first-run
---

# Codex as a harness

**Why:** the CEO, 2026-11-06: Codex CLI as a third [harness](../product/architecture/harness-adapters.md), run as the CEO runs it in a terminal ([`pcodex`](https://github.com/brickpop/pcodex)).

**Decided (the CEO, 2026-11-06):**
- **The browser login, for business accounts and the like**; the device-code login (`codex login --device-auth`) isn't one the CEO can use.
- **No port opened for its callback:** the Connect terminal asks for the address the browser lands on and hands it to Codex inside the container ([Harness adapters → Codex](../product/architecture/harness-adapters.md#codex)).
- **The CEO's `~/.codex` stays out:** a profile has its own login, as Claude Code's do.
- **No Codex code for moving conversations**, a legacy path ([Remove the workspace migration](./remove-workspace-migration.md)): a Codex agent whose sandbox moves starts fresh.

**Probed** on Codex 0.167.0 in a terminal against a stand-in Responses API server and a stand-in MCP server (2026-11-06): the trust dialog skipped with the sandbox trusted; hooks run only when trusted (`--dangerously-bypass-hook-trust`); the hooks' payloads (`Bash` for its command tool, `mcp__foreman__<tool>`, `request_user_input`, `Interrupt` on Esc and no `Stop`); the question tool refused outside plan mode without `default_mode_request_user_input`; `AGENTS.md` in `CODEX_HOME` reaches the model; `resume --last -- <opener>`; Ctrl+J is a new line; an idle session keeps only its MCP server. The adapter's own generated launch ran real Codex end to end. The login wrapper relayed a pasted callback to Codex's server (refused, as expected, for a made-up `state`). How `auth.json` is saved and shared was read in Codex's code.

- [x] the probe, its results in [Harness adapters → Codex](../product/architecture/harness-adapters.md#codex)
- [x] `internal/harness/codex`: Launch, Login (the callback relay), Interpret, Asks, Describe, Connected, Newline; a launch file may be a link (`harness.File.Link`); `harness.Questions`, `MCPTool` and `Relative` shared with Claude Code's and Pi's adapters
- [x] Codex in the agent image (after Pi), the adapter in `main.go`; `commit` drops Codex's `Generated with` line
- [x] the Connect dialog's and Profiles' texts
- [x] tests: the launch (home, prompt, config, link, mounts, flags, continue and opener), the shared login through the link, the login, the hooks read, the attribution
- [ ] **the CEO's check:** a Codex profile connected (the pasted callback), a deploy, the team tools called, the activity lines and a question, a restart that continues, two agents on one login across a token refresh
- [ ] a real-Codex integration test like `just test-claude`
