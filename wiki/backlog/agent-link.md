---
type: task
status: in-progress
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Agent link: foreman-agent, the agent port, and hooks into statuses

**Why:** every team tool and every "is it idle?" answer travels this link ([Building blocks → 4](../product/architecture/building-blocks.md#4-agent-link-agentlink--foreman-agent), [Team tools → Transport](../product/architecture/team-tools.md#transport)). Nothing else in the epic works without it.

**Done when:** an agent started by Foreman has the `foreman` MCP server and Foreman's hooks configured; `foreman-agent`, mounted into its container, relays tool calls and hook events to Foreman's agent port with the instance's token; Foreman lists the tools the caller's role allows; Claude Code's hooks move the instance between `running`, `idle` and `awaiting`, shown live in the UI.

- [x] `cmd/foreman-agent`: `mcp` (a stdio MCP server, official Go SDK, relaying `tools/list` and `tools/call` to Foreman) and `hook <event>` (forwards the payload; never fails the harness)
- [x] carried in Foreman's binary (`agentbin/`, a Linux build for Foreman's CPU type, made by `just build` and `just dev`) and mounted read-only into every container
- [x] `internal/agentlink`: the agent port on the host's loopback, one token per instance (signed with the workspace's agent-link secret, accepted only while the instance runs), a caller's identity only from its token; tools and hooks handed to interfaces, so Messaging plugs in
- [x] the Claude Code adapter writes the MCP config (`--mcp-config`) and the hooks into the instance's settings, and gets the link's address and token in its environment
- [x] `Interpret`: `UserPromptSubmit`/`PreToolUse` → `running`; `Stop` → `idle`; `PreToolUse` of `AskUserQuestion`, or `Notification` with `permission_prompt` → `awaiting` ([experiment D](./prove-d-claude-delivery-hooks.md)); statuses as `instance.status` events
- [x] tests: the relay both ways, tokens (a wrong or revoked one is refused; no "from" is trusted), the hook-to-status mapping (tools by role come with the tools)
- [x] end to end on the real server (Direct engine): the real foreman-agent's hook and MCP `tools/list` reach the agent port with the lead's token; a forged token gets 401
- [ ] the CEO sees a real Claude Code's status move (running, idle, awaiting) in Sessions (`just dev`)

Not here: the tools themselves ([Messaging](./messaging-delivery.md), [Git tools](./git-tools.md)).
