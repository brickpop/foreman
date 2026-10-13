---
type: task
status: todo
priority: high
tags: [spike]
---

# Spike: reach Foreman's MCP server from a container

Prove a container can call an MCP server on the host on both Linux (pasta) and macOS (podman machine), via `host.containers.internal` and a per-instance bearer token, while the UI listener stays on localhost. Measure how long a harness lets an MCP tool call block (for `escalate(approval)`). Feeds [ACP](../product/architecture/acp.md) and [Security](../product/architecture/security.md).

- [ ] Linux rootless (pasta)
- [ ] macOS podman machine
- [ ] Tool-call timeout per harness (Claude Code, Codex)
- [ ] Write the findings into the specs, then delete this card
