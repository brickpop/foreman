---
type: task
status: done
priority: medium
tags: [feature]
epic: full-ui-demo
---

# Team and Agent pages, and the Deploy dialog

**Why:** reusable teams the CEO tunes and deploys ([Team page](../product/features/team-page.md), [Agent page](../product/features/agent-page.md), [Deploy a team](../product/features/deploy-team.md), [Avatar](../product/design/avatar.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** the Team page with its tabs (Deployments with Stop and its confirmation, Members, Playbook, File); the Agent page (Running on, the Look editor with Shuffle, harness, personality, skills, job description, saved state, the session prompt with its per-deployment switcher or placeholders); the Deploy dialog (team, project, goal).

- [x] Team page: header (purpose, where deployed, Hire, Deploy to a project), Deployments (working / waiting on you, Sessions, Stop with its confirmation and toast), Members (status per deployment), Playbook (who resolved; an unknown one flagged), File (the team file as on disk: `Team.source`)
- [x] Agent page: Running on, the Look editor (swatches, traits, Shuffle), harness with the custom command, personality, skills, job description; choices save at once, text on blur or Enter; "Saved" for 3 s; a conflicting save keeps the edit and offers Keep mine / Take theirs
- [x] session prompt view: per deployment with its switcher (`GET /api/instances/:id/prompt`, composed by the server), or the template with highlighted placeholders (`GET /api/teams/:id/agents/:agent/prompt`); both endpoints built
- [x] Deploy dialog (team or project preset, already-there disabled, goal required; opens Sessions on the lead; toast), from the Team page and Project settings
- [x] the CEO looks at it on the Mac (`just dev`): approved 2026-10-15
