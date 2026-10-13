---
type: spec
status: planned
tags: [model, agents, teams]
---

# Agent soul

One member of a [team definition](./team-definition.md): who the agent is (name, role, personality, skills, job description), which harness runs it, and how it looks. A soul is stored under `agents.<id>` of its team file and is shared by every [deployment](./deployment.md) of the team: editing it changes all of them. Working memory is not part of the soul; it is per [instance](./instance.md) and stays inside that sandbox ([Agent runtime](../architecture/agent-runtime.md)). Edited on the [Agent page](../features/agent-page.md) or via the [Chief of Staff](../features/chief-of-staff.md) (hire, refine).

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| (key) | String | yes | | slug, unique within the team | Agent id (see [Team definition](./team-definition.md#open-questions) on collisions) |
| `name` | String | yes | | unique across the workspace | Display name, used in @-mentions, tabs, prompts |
| `role` | String | yes | | enum: `team_lead`, `architect`, `engineer`, `qa`, `scrum_master`, `reviewer`, or a custom role | Labels: Team Lead, Architect, Engineer, QA / Tester, Scrum Master, Reviewer |
| `harness` | String | yes | | enum: `claude-code`, `codex`, `pi`, `hermes`, `custom` | Labels: Claude Code, Codex CLI, Pi, Hermes Agent, Custom ([Harness adapters](../architecture/harness-adapters.md)) |
| `command` | String | when `harness: custom` | `""` | | The custom command line (e.g. `./bin/loadbot --acp`) |
| `personality` | String | | | | One or a few lines |
| `skills` | Array of String | | `[]` | no duplicates | Shown as chips |
| `instruction` | String (multi-line) | | the role's default | | The job description |
| `look.color` | String (hex) | yes | random from palette | one of 8 background colors | [Avatar](../design/avatar.md) background |
| `look.skin` | String (hex) | yes | random | one of 5 | |
| `look.hair` | String | yes | random | enum: `short`, `long`, `curly`, `bun`, `bald` | |
| `look.hair_color` | String (hex) | yes | random | one of 5 | |
| `look.eyes` | String | yes | random | enum: `dots`, `glasses`, `shades`, `sleepy` | |
| `look.beard` | String | yes | random | enum: `none`, `stubble`, `full`, `mustache` | |

Palettes are defined in [Avatar](../design/avatar.md). A hired agent gets a random face.

## Default job descriptions

Used when a role is hired without a custom `instruction` (the CoS appends `Focus: <focus>.`).

| Role | Default `instruction` |
|---|---|
| Team Lead | Break the goal into cards on the board. Assign cards and review every handoff before integrating. You are the only member who opens PRs to main. |
| Engineer | Work only in your own worktree. Write the test first. Hand off to the lead with a summary and test results. |
| QA | Try to break whatever is handed to you: property tests, fuzzing, races. File every finding with a repro. Never fix the code yourself. |
| Scrum Master | Own ./wiki. Keep the board in sync with reality. Record decisions in ADRs. Ping owners of stale cards. |
| Architect | Propose designs as ADRs in wiki/adr/. Back every claim with a spike and numbers. |
| Reviewer | Review diffs and ADRs for correctness and clarity. Approve or request changes. Never push code. |

## Invariants

- Display names are unique across the workspace.
- The soul is the same in every deployment; only the composed [session prompt](../architecture/session-prompt.md) differs per project.
- A soul edit reaches each running instance as a context-update message on its next turn, and a full re-prompt at its next restart.

## Open questions

1. **Custom roles.** The PRD lists "custom" members but no field shape for a custom role (free text? does it get wiki write access?). Recommendation: free-text `role`; only `scrum_master` (or the lead fallback) gets wiki write access.
2. **Switching harness from a blocked request.** "Switch to Claude Code" on a [blocked request](../features/requests.md): does it edit the soul (all deployments) or override that one instance? Recommendation: an instance-level override recorded on the [instance](./instance.md), lasting until the instance is restarted from the team file, with an option to persist it to the soul; the CEO edits the soul on the [Agent page](../features/agent-page.md) for a permanent change.
