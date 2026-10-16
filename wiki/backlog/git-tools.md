---
type: task
status: in-progress
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Git tools: switch, commit, merge, abort_merge, handoff, and contribute

**Why:** agents' `.git` is read-only, so they branch, commit and merge through Foreman, which runs git on the host in the caller's worktrees ([Protocol → `switch`](../product/architecture/protocol.md#switch), [`commit`](../product/architecture/protocol.md#commit), [`merge`](../product/architecture/protocol.md#merge), [`abort_merge`](../product/architecture/protocol.md#abort_merge), [Sandboxes and git policy → Agents and git](../product/architecture/sandboxes-and-git.md#agents-and-git)).

Unblocked: [Agent link](./agent-link.md) and [Messaging](./messaging-delivery.md) are done.

**Done when:** `switch`, `commit` (the lead's also covers the live project repo), `merge` and `abort_merge` work in every code repo of the caller, with the branch policy; `handoff` names a committed branch and never commits; `contribute` files a note in `project/wiki/inbox/`, commits it as the agent and tells the lead.

- [x] Git: `Switch`, `CommitAll` (authored as the agent; finishes a merge once no markers are left), `CommitAs`, `Merge` (conflicts left in the files), `AbortMerge`, on the host
- [x] the tools and their errors (`uncommitted_changes`, `branch_not_allowed`, `conflicts_left`, …), added to `prompt.Offered`'s built list (the prompt and the router follow)
- [x] `handoff` (members only): refuses uncommitted work (commit first), checks the branch (`wt/<self>/*`, exists, has commits beyond `main`), delivers `Message msg-N from Tove (handoff of wt/tove/x): …`; checks and work items come with the [Git](./epics/git.md) epic
- [x] `contribute`: the inbox entry (frontmatter from the caller's token), its commit, the notice to the lead
- [x] the session prompt follows on its own (its Git section, branches and checks line follow the tools offered); the lead's branches line no longer names `request_merge` before it exists, the approvals note waits for `escalate`
- [x] tests: on real repos in a temp dir, every error, the lead's project-repo commit
- [x] decided with the CEO (2026-10-16): `update` became `merge` (the git word) with `abort_merge` to back out; agents never rebase (shared branches, tracking by commit); `handoff` no longer commits: `commit` is the one way
- [x] the team-tools router (`internal/teamtools`): caller from the token, the tools offered by role, one block per tool (Messaging, Workflow); its test keeps the served tools and the prompt in step
- [x] found on the way and fixed: a stop while an agent was still starting kept its slot, so a redeploy right after never started it (the flaky `TestStopKeepsTheWorktrees`); now reproduced exactly by a test
- [x] every git tool call is kept as a team action (migration 004, `GET /api/actions`, `team.action`) and shown in Team traffic, refusals included (the CEO's decision, 2026-10-16)
- [ ] the CEO sees Tove commit and hand off to Mara, and Mara merge it with `merge` (`just dev`)
