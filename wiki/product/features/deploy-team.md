---
type: spec
status: planned
tags: [feature, ui, teams, runtime]
---

# Deploy a team

A modal ([App shell](./app-shell.md#overlays)) that assigns a team to a project, with an optional goal, creating a [deployment](../models/deployment.md). Opened from the [Team page](./team-page.md) (team preset), [Project settings](./project-settings.md) (project preset), empty [Sessions](./sessions.md) (nothing preset), or the [Chief of Staff](./chief-of-staff.md) after creating a team.

## Content

- Title `Deploy <Team>`, or `Deploy a team` with no team preset. Subline: `Starts a separate set of sessions for this project. Deployments on other projects aren't affected.`
- **Team** option group, only without a team preset.
- **Project** option group. A project where the selected team is already deployed is disabled and labelled `<project> (already there)`.
- **"Goal on this project (optional)"**: textarea (3 rows), placeholder `What does done look like? Or leave it empty and give it to the lead in its session.` Optional (the CEO, 2026-10-16): when the project is already defined, the CEO may prefer to give the goal in the lead's session.
- Buttons: `Cancel`, and primary **`Deploy`**, disabled until team and project are set and the pair is new.

## Effect

`POST /api/deployments {team, project, goal?}` (a blank goal is none) ([API](../architecture/api.md)). Existing deployments are untouched.

It is refused, with the reason shown in the dialog and nothing created, before Podman runs (`409 NOT_READY`; the dialog says so beforehand, linking to the setup on the Dashboard, [First run](./first-run.md#deploying)), when the team or project has problems, when the project is archived, or when the project's [profile](../architecture/session-runner.md#credentials) doesn't exist, runs a harness that isn't supported yet, or has no login yet (`profile work isn't connected yet: Connect it under Settings → Agents → Profiles`).

The dialog doesn't wait for that refusal: once a project is chosen, one row names the profile its agents run on, `Its agents run on the <profile> profile (<Harness>).` (the CEO, 2026-10-18: the dialog picks nothing; the project's settings do). When it isn't connected, the row says `…, which isn't connected yet, so they can't start.` with a small **`Connect`** button that opens the Connect dialog ([Settings → Profiles](./settings.md#profiles)) over this one; when it doesn't exist, `The <profile> profile doesn't exist: pick another in the project's settings.` Deploy stays disabled until it is connected.

1. The deployment is created; every member gets an [instance](../models/instance.md). Only the lead starts: `starting` (or `queued` when the session limit is reached, [Agent runtime](../architecture/agent-runtime.md#concurrency)) with the task `Cloning <repos>`. The other members are `asleep` until they have work ([Lazy starts](../architecture/agent-runtime.md#lazy-starts)).
2. The lead's worktrees are created and its session starts ([Sandboxes and git policy](../architecture/sandboxes-and-git.md), [Agent runtime](../architecture/agent-runtime.md)). The first lines of a session read `<harness> · <workspace>/agents/<agent>` and `Worktrees of <repos> · reading project/wiki`.
3. The dialog closes, Foreman opens [Sessions](./sessions.md) on the new deployment with the lead's pane (the next thing the CEO does is talk to the lead), the "deployed" toast appears (click opens the lead's session), and the lead is already working: its harness started with the goal as its opening message. Without a goal it starts with no opening message and waits for the CEO's.
