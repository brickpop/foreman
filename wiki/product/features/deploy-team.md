---
type: spec
status: planned
tags: [feature, ui, teams, runtime]
---

# Deploy a team

A modal ([App shell](./app-shell.md#overlays)) that assigns a team to a project, creating a [deployment](../models/deployment.md). A project has one team at most: deploying another one **replaces** the team there (the CEO, 2026-10-31). Opened from the [Team page](./team-page.md) and a Dashboard team card (team preset), the Dashboard header (nothing preset), or a project's [Session](./sessions.md) with no team and its [Deployment](./project-deployment.md) section (project preset, and the team that worked on the project last chosen at first, still changeable, when none is there now).

## Content

- Title `Deploy <Team>`, or `Deploy a team` with no team preset. Subline: `A project has one team at most. The team's work here is separate from its work on other projects.`
- **Team** option group, only without a team preset.
- **Project** option group. A project where the selected team is already deployed is disabled and labelled `<project> (already there)`; one with another team on it carries that team's name (`<project> · <Team>`, the name in `--tx4`).
- **Replacing**: when the project chosen has another team, an accent box (`--accSoft`, `--accLine`, `--accTx`) says so before Deploy: `Replaces <Team> on <project>: <n> sessions end. Worktrees and branches are kept.`
- Buttons: `Cancel`, and primary **`Deploy`**, disabled until team and project are set and the pair is new.

## Effect

`POST /api/deployments {team, project}` ([API](../architecture/api.md)). The team on the project, if another, is stopped first, as by Stop ([Deployment → Lifecycle](../models/deployment.md#lifecycle)); the team's deployments on other projects are untouched. A refused deploy replaces nothing.

It is refused, with the reason shown in the dialog and nothing created, before Podman runs (`409 NOT_READY`; the dialog says so beforehand, linking to the setup on the Dashboard, [First run](./first-run.md#deploying)), when the team or project has problems, when the project is archived, or when the project's [profile](../architecture/session-runner.md#credentials) doesn't exist, runs a harness that isn't supported yet, or has no login yet (`profile work isn't connected yet: Connect it under Settings → Agents → Profiles`).

The dialog doesn't wait for that refusal: once a project is chosen, one row names the profile its agents run on, `Its agents run on the <profile> profile (<Harness>).` (the CEO, 2026-10-18: the dialog picks nothing; the project's settings do). When it isn't connected, the row says `…, which isn't connected yet, so they can't start.` with a small **`Connect`** button that opens the Connect dialog ([Settings → Profiles](./settings.md#profiles)) over this one; when it doesn't exist, `The <profile> profile doesn't exist: pick another in the project's settings.` Deploy stays disabled until it is connected.

An **archived** project reads `<id> (archived)` among the projects, and once chosen a row says `<id> is archived, so no team can be deployed to it.` with a small **`Unarchive`** button, the same as its [Project settings](./project-settings.md) (the CEO, 2026-10-30: say so before Deploy, not as its refusal). Deploy stays disabled until it is unarchived.

1. The deployment is created; every member gets an [instance](../models/instance.md). Only the lead starts: `starting` (or `queued` when the session limit is reached, [Agent runtime](../architecture/agent-runtime.md#concurrency)) with the task `Cloning <repos>`. The other members are `asleep` until they have work ([Lazy starts](../architecture/agent-runtime.md#lazy-starts)).
2. The lead's worktrees are created and its session starts ([Sandboxes and git policy](../architecture/sandboxes-and-git.md), [Agent runtime](../architecture/agent-runtime.md)). The first lines of a session read `<harness> · <workspace>/agents/<agent>` and `Worktrees of <repos> · reading project/wiki`.
3. The dialog closes, Foreman opens the project's [Session](./sessions.md) on the lead (the next thing the CEO does is talk to the lead), the "deployed" toast appears (click opens the lead's session), and the lead waits for the CEO's first message: no goal is asked here, since it is the project's, kept in its wiki, and changes over time; the lead asks for it when there is none ([Session prompt](../architecture/session-prompt.md)) (the CEO, 2026-10-31).
