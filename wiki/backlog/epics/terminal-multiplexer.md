---
type: epic
status: todo
order: 1
---

# Terminal multiplexer

**Goal:** a working multiplexer of isolated agents. Projects (their wiki, board and definition) get teams (souls and skills, as team files), a deployment starts one agent per member in its own Podman container, and the CEO watches and talks to each of them in the browser. No messaging between agents yet: the CEO talks to every agent directly.

**Usable when done:** start Foreman, prepare images, log in to Claude Code, add a project, read its wiki and board, edit a team's souls, deploy the team with a goal, and work with each agent in Sessions (tabs, split, reload with an exact snapshot); stop and restart Foreman without losing the agents.

**Journey steps:** 1–6, 8–11, 15, 20, 28–30 of [the journey](../../product/journey.md). Step 7 (creating a team) is done by editing the team file by hand until the Chief of Staff (epic 4).

**How agents get the project's code (decided):** every agent's container mounts the project folder itself, read-write: the CEO's checkout (e.g. `ledger/ledger-api`). No clones, no per-agent branches in this epic; they arrive with the [Git](./git.md) epic. Accepted costs until then:
- the agents of a deployment, and two deployments on one project, share one working tree and can collide (two agents editing the same files, one switching branches under another);
- agents edit the CEO's checkout directly.

In this epic there is no messaging yet: every agent works only on what the CEO types into it, and **the only protection against collisions is a warning** on the pane when the CEO types into an agent while another agent of the same project is busy ([Sessions → Talking to the team](../../product/features/sessions.md#talking-to-the-team)). Messaging (epic 2) adds enforced turns per project; the Git epic removes the need for them.

**Blocked by:** [Prove](./prove.md) (experiments A, B and E), and [the decisions on the journey's gaps](../decide-journey-gaps.md).

**Exit criteria:** the steps above work end to end on macOS and Fedora with Claude Code, covered by tests (Go and UI) and by a written manual check on both machines; the specs they touch are `built` or `partial` with the gaps named.
