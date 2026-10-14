---
type: spec
status: planned
tags: [feature, ui, teams, runtime]
---

# Deploy a team

A modal ([App shell](./app-shell.md#overlays)) that assigns a team to a project with a goal, creating a [deployment](../models/deployment.md). Opened from the [Team page](./team-page.md) (team preset), [Project settings](./project-settings.md) (project preset), or the [Chief of Staff](./chief-of-staff.md) after creating a team.

## Content

- Title `Deploy <Team>`, or `Deploy a team` with no team preset. Subline: `Starts a separate set of sessions for this project. Deployments on other projects aren't affected.`
- **Team** option group, only without a team preset.
- **Project** option group. A project where the selected team is already deployed is disabled and labelled `<project> (already there)`.
- **"Goal on this project"**: textarea (3 rows), placeholder `What does done look like?`. Required.
- Buttons: `Cancel`, and primary **`Deploy`**, disabled until team, project and a non-blank goal are set and the pair is new.

## Effect

`POST /api/deployments {team, project, goal}` ([API](../architecture/api.md)). Existing deployments are untouched.

1. The deployment is created; every member gets an [instance](../models/instance.md) in `starting` (or `queued` when the session limit is reached, [Agent runtime](../architecture/agent-runtime.md#concurrency)) with the task `Cloning <repos>`.
2. Worktrees are created and sessions start ([Sandboxes and git policy](../architecture/sandboxes-and-git.md), [Agent runtime](../architecture/agent-runtime.md)). The first lines of each session read `<harness> · <project>/.sandboxes/<agent>` and `Worktrees of <repos> · reading project/wiki`.
3. The dialog closes, Foreman opens [Sessions](./sessions.md) on the new deployment with the lead's pane (the next thing the CEO does is talk to the lead), the "deployed" toast appears (click opens the lead's session), and the lead receives the goal.
