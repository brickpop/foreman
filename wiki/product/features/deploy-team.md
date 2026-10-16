---
type: spec
status: planned
tags: [feature, ui, teams, runtime]
---

# Deploy a team

A modal ([App shell](./app-shell.md#overlays)) that assigns a team to a project with a goal, creating a [deployment](../models/deployment.md). Opened from the [Team page](./team-page.md) (team preset), [Project settings](./project-settings.md) (project preset), empty [Sessions](./sessions.md) (nothing preset), or the [Chief of Staff](./chief-of-staff.md) after creating a team.

## Content

- Title `Deploy <Team>`, or `Deploy a team` with no team preset. Subline: `Starts a separate set of sessions for this project. Deployments on other projects aren't affected.`
- **Team** option group, only without a team preset.
- **Project** option group. A project where the selected team is already deployed is disabled and labelled `<project> (already there)`.
- **"Goal on this project"**: textarea (3 rows), placeholder `What does done look like?`. Required.
- Buttons: `Cancel`, and primary **`Deploy`**, disabled until team, project and a non-blank goal are set and the pair is new.

## Effect

`POST /api/deployments {team, project, goal}` ([API](../architecture/api.md)). Existing deployments are untouched.

It is refused, with the reason shown in the dialog and nothing created, before Podman runs (`409 NOT_READY`; the dialog says so beforehand, linking to the setup on Home, [First run](./first-run.md#deploying)), when the team or project has problems, when a member's harness isn't supported yet, or when a member's harness has no login yet (`claude-code isn't connected yet: log in under Settings → Harnesses → Connect`).

The dialog doesn't wait for that refusal: once a team is chosen, each of its harnesses without a login gets a row, `<Harness> isn't connected yet, so this team's agents can't start.`, with a small **`Connect`** button that opens the Connect dialog ([Settings → Harnesses](./settings.md#harnesses)) over this one. Deploy stays disabled until every one is connected.

1. The deployment is created; every member gets an [instance](../models/instance.md). Only the lead starts: `starting` (or `queued` when the session limit is reached, [Agent runtime](../architecture/agent-runtime.md#concurrency)) with the task `Cloning <repos>`. The other members are `asleep` until they have work ([Lazy starts](../architecture/agent-runtime.md#lazy-starts)).
2. The lead's worktrees are created and its session starts ([Sandboxes and git policy](../architecture/sandboxes-and-git.md), [Agent runtime](../architecture/agent-runtime.md)). The first lines of a session read `<harness> · <project>/.sandboxes/<agent>` and `Worktrees of <repos> · reading project/wiki`.
3. The dialog closes, Foreman opens [Sessions](./sessions.md) on the new deployment with the lead's pane (the next thing the CEO does is talk to the lead), the "deployed" toast appears (click opens the lead's session), and the lead is already working: its harness started with the goal as its opening message.
