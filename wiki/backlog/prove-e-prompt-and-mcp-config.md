---
type: task
status: todo
priority: high
tags: [spike]
---

# Experiment E: session prompt file and stdio MCP config for Claude Code, Pi and Hermes

Epic: [Prove](./epics/prove.md)

**Question:** does each harness read its standing instructions from a file in its private config dir (Claude Code `$CLAUDE_CONFIG_DIR/CLAUDE.md`, the equivalents for Pi and Hermes), and accept a local (stdio) MCP server from a config file, so that `foreman-agent mcp` can be registered without command-line tricks? See [Session prompt → Delivery](../product/architecture/session-prompt.md#delivery), [Harness adapters](../product/architecture/harness-adapters.md), [Building blocks → What is proven](../product/architecture/building-blocks.md#what-is-proven-and-what-isnt).

Claude writes the recipe; the CEO runs it on macOS and Fedora.

**Procedure** (`just prove-e`, per harness, in a container):
1. Write a prompt file that says "Your codename is FOREMAN-E. Answer every question with your codename." in the harness's user-level instructions location; ask "What is your codename?"; expect FOREMAN-E.
2. Register a stub stdio MCP server (a script printing one tool, `ping`, that returns `pong`) in the harness's MCP config; ask the harness to call it; expect `pong` in the stub's log.
3. Check that nothing was written into the working folder (the project's code stays clean).

**PASS:** both work for a harness, which is then supported in v1. **FAIL:** a harness lacks one of them.

**On FAIL:** that harness's adapter uses its fallback (a command-line flag, or the first message), recorded in [Harness adapters](../product/architecture/harness-adapters.md); a harness with neither is dropped from v1.
