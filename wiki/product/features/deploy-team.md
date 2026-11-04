---
type: spec
status: planned
tags: [feature, ui, teams, runtime]
---

# Deploy a team

A modal ([App shell](./app-shell.md#overlays)) that assigns a team to a project, creating a [deployment](../models/deployment.md). A project has one team at most: deploying another one **replaces** the team there (the CEO, 2026-10-31). Opened from the [Team page](./team-page.md) and a Dashboard team card (team preset), the Dashboard header (nothing preset), or a project's [Session](./sessions.md) with no team and its [Deployment](./project-team.md) section (project preset, and the team that worked on the project last chosen at first, still changeable, when none is there now).

## Content

- Title `Deploy <Team>`, or `Deploy a team` with no team preset. Subline: `A project has one team at most. The team's work here is separate from its work on other projects.`
- **Team** option group, only without a team preset.
- **Project** option group. A project where the selected team is already deployed is disabled and labelled `<project> (already there)`; one with another team on it carries that team's name (`<project> · <Team>`, the name in `--tx4`).
- **Replacing**: when the project chosen has another team, an attention box (`--attnSoft`, `--attnLine`, `--attnTx`) says so before Deploy: `Replaces <Team> on <project>: <n> sessions end. Worktrees and branches are kept.`
- **Earlier conversations**: when the team's agents already have conversations on the chosen project (it worked there before), a checkbox, off by default: `Start fresh: the agents don't continue their earlier conversations here (kept on disk).` Unchecked, each agent continues its last conversation on the project (the CEO, 2026-11-02). Not shown when there are none (`GET /api/deployments/continuing`). Built 2026-11-03.
- Buttons: `Cancel`, and primary **`Deploy`**, disabled until team and project are set and the pair is new.

## Effect

`POST /api/deployments {team, project}` ([API](../architecture/api.md)). The team on the project, if another, is stopped first, as by Stop ([Deployment → Lifecycle](../models/deployment.md#lifecycle)); the team's deployments on other projects are untouched. A refused deploy replaces nothing.

It is refused, with the reason shown in the dialog and nothing created, before Podman runs (`409 NOT_READY`; the dialog says so beforehand, linking to the setup on the Dashboard, [First run](./first-run.md#deploying)), when the team or project has problems, when the project is archived, or when the project's [profile](../architecture/session-runner.md#credentials) doesn't exist, runs a harness that isn't supported yet, or has no login yet (`profile work isn't connected yet: Connect it under Settings → Agents → Profiles`).

The dialog doesn't wait for that refusal: once a project is chosen, a **Profile** group like Team and Project above it picks the profile its agents run on: a chip per profile of the CEO's (`<name>` in Mono, `· <Harness>` in `--tx4`), the project's own pressed (one that no longer exists stays, pressed, with an `--err` border), then a line under them (the CEO, 2026-11-04: chips, not a box with a select). It is the **project's** profile, as in [Project settings](./project-settings.md) (one per project): another picked here becomes the project's on Deploy (`PUT /api/projects/{id}/profile`, then the deploy), so Cancel changes nothing, and the line says `It becomes the project's profile on Deploy.` (otherwise `The project's profile: its agents run on it.`) (the CEO, 2026-11-04; it was the settings' only until then). A profile on another harness than the project's offers no Start fresh: no agent continues a conversation across harnesses. When the profile isn't connected, the line says `Not connected yet, so its agents can't start.` with a small **`Connect`** button that opens the Connect dialog ([Settings → Profiles](./settings.md#profiles)) over this one; when it doesn't exist, `That profile doesn't exist: pick another.` Deploy stays disabled until it is connected.

An **archived** project reads `<id> (archived)` among the projects, and once chosen a row says `<id> is archived, so no team can be deployed to it.` with a small **`Unarchive`** button, the same as its [Project settings](./project-settings.md) (the CEO, 2026-10-30: say so before Deploy, not as its refusal). Deploy stays disabled until it is unarchived.

1. The deployment is created; every member gets an [instance](../models/instance.md). Only the lead starts: `starting` (or `queued` when the session limit is reached, [Agent runtime](../architecture/agent-runtime.md#concurrency)) with the task `Cloning <repos>`. The other members are `asleep` until they have work ([Lazy starts](../architecture/agent-runtime.md#lazy-starts)).
2. The lead's worktrees are created and its session starts ([Sandboxes and git policy](../architecture/sandboxes-and-git.md), [Agent runtime](../architecture/agent-runtime.md)). The first lines of a session read `<harness> · <workspace>/agents/<agent>` and `Worktrees of <repos> · reading project/wiki`.
3. The dialog closes, Foreman opens the project's [Session](./sessions.md) on the lead (the next thing the CEO does is talk to the lead), the "deployed" toast appears (click opens the lead's session), and the lead waits for the CEO's first message: no goal is asked here, since it is the project's, kept in its wiki, and changes over time; the lead asks for it when there is none ([Session prompt](../architecture/session-prompt.md)) (the CEO, 2026-10-31).
