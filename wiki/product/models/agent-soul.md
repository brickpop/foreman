---
type: spec
status: planned
tags: [model, agents, teams]
---

# Agent soul

One member of a [team definition](./team-definition.md): who the agent is (name, role, personality, skills, job description), and how it looks (what it runs on is its project's profile: [below](#what-it-runs-on)). A soul is stored under `agents.<id>` of its team file and is shared by every [deployment](./deployment.md) of the team: editing it changes all of them. Working memory is not part of the soul; it is per [instance](./instance.md) and stays inside that sandbox ([Agent runtime](../architecture/agent-runtime.md)). Edited on the [Agent page](../features/agent-page.md) or via the [Chief of Staff](../features/chief-of-staff.md) (hire, refine).

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| (key) | String | yes | | slug, unique across all teams | Agent id: the slug of the display name ([Team definition](./team-definition.md#schema)) |
| `name` | String | yes | | unique across all teams | Display name, used in @-mentions, tabs, prompts |
| `role` | String | yes | | `team_lead`, `architect`, `engineer`, `qa`, `reviewer`, or a custom role in snake_case (e.g. `security_auditor`) | Labels: Team Lead, Architect, Engineer, QA / Tester, Reviewer; a custom role is shown with underscores as spaces, capitalised ("Security auditor"). Snake_case so a playbook can name it as a `who` ([Team definition](./team-definition.md#playbook-and-escalation)). No role but the lead's grants write access to the knowledge base |
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
| Team Lead | Break the goal into cards on the board. Assign cards and review every handoff before integrating. You are the only member who requests merges to main (request_merge, with the card that explains the work). |
| Engineer | Work only in your own worktree. Write the test first. Hand off to the lead with a summary and test results. |
| QA | Try to break whatever is handed to you: property tests, fuzzing, races. File every finding with a repro. Never fix the code yourself. |
| Architect | Propose designs as ADRs with contribute (kind proposal): only the lead writes the knowledge base. Back every claim with a spike and numbers. |
| Reviewer | Review diffs and ADRs for correctness and clarity. Approve or request changes. Never push code. |

## Invariants

- Display names are unique across all teams.
- The soul is the same in every deployment; only the composed [session prompt](../architecture/session-prompt.md) differs per project.
- A soul edit reaches each running instance as a context-update message on its next turn, and a full re-prompt at its next restart.

## What it runs on

A soul names no harness (the CEO, 2026-10-18): every agent deployed on a project runs on that project's [profile](../architecture/session-runner.md#credentials), a harness and its login. A team file from before, with `harness` and `command` lines, still loads: they are ignored and dropped by the next save. Choosing another profile for one agent (the "Switch harness" of a blocked request, a lead on Claude Code and the rest on Pi) is [Profiles per agent](../../backlog/profiles-per-agent.md), later.
