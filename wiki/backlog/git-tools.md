---
type: task
status: todo
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Git tools: switch, commit, update, handoff, and contribute

**Why:** agents' `.git` is read-only, so they branch, commit and merge through Foreman, which runs git on the host in the caller's worktrees ([Protocol → `switch`](../product/architecture/protocol.md#switch), [`commit`](../product/architecture/protocol.md#commit), [`update`](../product/architecture/protocol.md#update), [Sandboxes and git policy → Agents and git](../product/architecture/sandboxes-and-git.md#agents-and-git)).

Unblocked: [Agent link](./agent-link.md) and [Messaging](./messaging-delivery.md) are done.

**Done when:** `switch`, `commit` (the lead's also covers the live project repo) and `update` work in every code repo of the caller, with the branch policy; `handoff` commits pending work first; `contribute` files a note in `project/wiki/inbox/`, commits it as the agent and tells the lead.

- [ ] Git: `Switch`, `Commit` (authored as the agent), `Update` (conflicts left in the files), on the host
- [ ] the tools and their errors (`uncommitted_changes`, `branch_not_allowed`, `conflicts_left`, …), added to `prompt.Offered`'s built list (the prompt and the router follow)
- [ ] `handoff` (members only): commits pending work, checks the branch (`wt/<self>/*`, exists, has commits beyond `main`), delivers `Message msg-N from Tove (handoff of wt/tove/x): …`; checks and work items come with the [Git](./epics/git.md) epic
- [ ] `contribute`: the inbox entry (frontmatter from the caller's token), its commit, the notice to the lead
- [ ] the session prompt follows on its own (its Git section, branches and checks line follow the tools offered)
- [ ] tests: on real repos in a temp dir, every error, the lead's project-repo commit
