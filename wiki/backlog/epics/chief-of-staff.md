---
type: epic
status: todo
order: 5
---

# Chief of Staff

**Goal:** the agent that manages the CEO's teams: creating them from a description, hiring, refining souls (team files, in the workspace repo). A real terminal on its own page; it proposes with `propose_team`, Foreman shows the proposal in a preview and writes it, one commit, only on the CEO's click. No branches or merge requests: those are for code (the CEO, 2026-10-18; [Chief of Staff](../../product/features/chief-of-staff.md)).

**Usable when done:** a team is created, hired into and tuned entirely by talking to the Chief of Staff.

**Journey steps:** 7 and 25 of [the journey](../../product/journey.md); 27 (project config) later.

**Cards:** [Create a team, hire, refine](../cos-create-team.md) first (pulled ahead of the Workflow epic by the CEO, 2026-10-18: it needs neither Workflow nor Git); then [Project config with the Chief of Staff](../cos-project-config.md) and the decision on [Starter teams](../starter-teams.md).

**Not blocked:** gap G2 (how it runs) is decided.

**Exit criteria:** the steps above work end to end on the real server, with tests on the fake harness.
