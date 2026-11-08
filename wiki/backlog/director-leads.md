---
type: task
status: blocked
priority: high
tags: [feature]
epic: director
---

# The Director and the leads

**Why:** the CEO, 2026-11-05: "I have one interaction, and this triggers work in many projects at once." The [Director](./epics/director.md) asks each involved project's lead to do its part and collects their reports. Dispatch and report only.

Blocked by [The Director as a standing agent](./director-standing.md).

**Decided:**

- **`ask_lead(project, request)`** (Director only): an `ask` to the lead of the project's active deployment, stored in **that deployment's** messages (its `msg-N`), from `director`, so the project's Activity shows it like any message. No team on the project: refused, saying so (the Director proposes a deployment or tells the CEO). The lead gets it as any delivery: `Message msg-12 from the Director (ask): …`, waking it if asleep.
- **Only leads.** The Director has no way to message members.
- **Several replies to a Director ask.** The lead answers with `reply(id, output, artifact?, done?)`, as often as it has something worth reporting (progress, a block, a branch in review); `done: true` ends it, and a reply after that is refused (`already_replied`). A teammate's ask is unchanged: its one reply ends it.
- **Leads never start a conversation with the Director** (the CEO, 2026-11-08): no tool reaches it except replying to its ask, so work the CEO starts in a lead's session has no thread to the Director. `ask("director", …)` is an unknown teammate.
- **What the Director sees:** deliveries qualified by project, `Message ledger:msg-12 from Rhea, lead of ledger (reply, progress): …`, read in full with `read_message("ledger:msg-12")`.
- **`projects()`** (Director only, read-only, wakes nobody): each project, its team, its lead's status, and the Director's open asks there.
- **When a team goes:** stopping or replacing a project's deployment expires the Director's open asks there, and the Director is told (`From Foreman: ledger's team was stopped; your ask ledger:msg-12 is closed.`).
- **The leads' prompt:** a Director ask is work the CEO delegated; the CEO wins on a conflict; report by replying to that ask, progress when meaningful, `done` at the end; questions and approvals still go to the CEO with `escalate`, never to the Director.
- **The Director's prompt:** it never answers a lead's request for the CEO; it tells the CEO what needs them.
- **Security, accepted (the CEO, 2026-11-05):** the Director is the one path between projects. [Security](../product/architecture/security.md) lists it with its mitigations (read-only HQs, no code or git, replies quoted and capped, its messages in each project's Activity), and [Team tools](../product/architecture/team-tools.md)' "cross-deployment messaging is deliberately disallowed" is amended to name this one exception.

**Steps:**

- [ ] messaging: a sender outside the deployment, qualified refs for the Director, repeated replies until `done`, expiry on stop and replace
- [ ] tools: `ask_lead`, `projects`, `reply`'s `done`; offered by role (`prompt.Offered`)
- [ ] prompts: the lead's paragraph, the Director's rules
- [ ] UI: the Director as a sender in Activity and in the lead's session markers
- [ ] specs: [Team tools](../product/architecture/team-tools.md), [Protocol](../product/architecture/protocol.md), [Team message](../product/models/team-message.md), [Session prompt](../product/architecture/session-prompt.md), [Security](../product/architecture/security.md), the Director's, a cross-project story in [the journey](../product/journey.md) and [use cases](../product/use-cases.md)
- [ ] tests: no team refused; several replies then `done`, then refused; a lead can't reach the Director unasked; a member can't be asked; stop and replace expire asks and tell the Director; a reply wakes a sleeping Director; output caps
