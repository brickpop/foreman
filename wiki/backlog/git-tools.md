---
type: task
status: todo
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Git tools: switch, commit, update, and contribute

**Why:** agents' `.git` is read-only, so they branch, commit and merge through Foreman, which runs git on the host in the caller's worktrees ([Protocol → `switch`](../product/architecture/protocol.md#switch), [`commit`](../product/architecture/protocol.md#commit), [`update`](../product/architecture/protocol.md#update), [Sandboxes and git policy → Agents and git](../product/architecture/sandboxes-and-git.md#agents-and-git)).

Blocked by [Agent link](./agent-link.md).

**Done when:** `switch`, `commit` (the lead's also covers the live project repo) and `update` work in every code repo of the caller, with the branch policy; `handoff` commits pending work first; `contribute` files a note in `project/wiki/inbox/`, commits it as the agent and tells the lead.

- [ ] Git: `Switch`, `Commit` (authored as the agent), `Update` (conflicts left in the files), on the host
- [ ] the tools and their errors (`uncommitted_changes`, `branch_not_allowed`, `conflicts_left`, …)
- [ ] `contribute`: the inbox entry (frontmatter from the caller's token), its commit, the notice to the lead
- [ ] the session prompt's Git section switches to the tools once they exist
- [ ] tests: on real repos in a temp dir, every error, the lead's project-repo commit
