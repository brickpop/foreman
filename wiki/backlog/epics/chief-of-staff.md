---
type: epic
status: todo
order: 5
---

# Chief of Staff

**Goal:** the agent that sets Foreman up for the CEO: creating teams from the example rosters, hiring, refining souls (team files, in the workspace repo) and changing project config (`project.toml`, in the project repo), always on a branch that comes back through `request_merge` as a review, like any merge request: Merge to main, Create PR on the forge of the repo it changes (the workspace repo's `workspace_forge`, the project repo's `forge.project_repo`), or Reject ([Pipeline and merges](../../product/features/pipeline-and-merges.md#review-and-merge)).

**Usable when done:** a team is created, hired into and tuned entirely by talking to the Chief of Staff; team and config changes arrive as reviews.

**Journey steps:** 7, 25 and 27 of [the journey](../../product/journey.md).

**Blocked by:** [Workflow with the CEO](./workflow.md), and the decision on how the Chief of Staff runs (gap G2 of the journey).

**Also:** the server side of the drawer's client area, which says "Not built yet" today ([technical debt](../../technical-debt.md#the-chief-of-staff-is-typed-in-the-ui-only)).

**Exit criteria:** the steps above work end to end; until then, teams are created by editing team files by hand.
