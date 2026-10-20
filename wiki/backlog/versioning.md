---
type: task
status: todo
priority: medium
tags: [decision]
epic: terminal-multiplexer
---

# Versioning: Foreman and foreman-agent, state.db, team-file format

Three things carry a version and can drift apart, and the CEO decides what happens when they do. Once decided, the answers go into the specs below and the implementation steps run in this epic.

1. **Foreman ↔ `foreman-agent`.** Running containers keep the `foreman-agent` they started with, while Foreman itself can be upgraded and re-attach to them ([Agent link](../product/architecture/building-blocks.md#4-agent-link-agentlink--foreman-agent), [Protocol](../product/architecture/protocol.md)). Proposal: a handshake on every connection with both versions and a protocol number; on a mismatch, Foreman either keeps talking (compatible protocol numbers) or marks the instance for restart with the new binary. *Open: refuse, restart automatically, or ask the CEO?*
2. **`state.db` schema.** Numbered migrations through `PRAGMA user_version` ([Persistence](../product/architecture/persistence.md), [State store](./state-store.md)). *Open: what an older Foreman does with a newer database (refuse to start, recommended), and whether to back up `state.db` before migrating.*
3. **Team-file format** (and `project.toml`, `settings.toml`): files the CEO and the Chief of Staff edit by hand ([Team definition](../product/models/team-definition.md), [Project config](../product/models/project-config.md)). *Open: a `version:` key in each file, and whether Foreman rewrites old files to the new format or only reads them.*

- [ ] decide 1–3 with the CEO; record in the specs
- [ ] handshake in `foreman-agent` and the agent link (when the agent link is built, epic 2)
- [ ] migration policy in the state store
- [ ] file format versions in home & config loading
