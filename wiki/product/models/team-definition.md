---
type: spec
status: planned
tags: [model, teams, agents, config]
---

# Team definition

`~/agents-ws/.foreman/teams/<team>.yaml`: a reusable roster of [agent souls](./agent-soul.md) plus the team's playbook and escalation rules. One definition can be [deployed](./deployment.md) to any number of projects; every deployment uses the same souls and the same playbook. Foreman owns these files: the UI ([Team page](../features/team-page.md), [Agent page](../features/agent-page.md)), the [Chief of Staff](../features/chief-of-staff.md) and the CEO's editor all write them. A change applies to every running instance on its next turn ([Agent runtime](../architecture/agent-runtime.md)).

## Example

```yaml
team: atlas
name: Atlas
purpose: Builds features end to end, tested and reviewed.   # optional, one line
lead: mara
agents:
  mara:
    name: Mara
    role: team_lead
    harness: claude-code   # claude-code | codex | pi | hermes | custom
    command: ""            # required when harness = custom
    personality: Calm, decisive, terse. Prefers small PRs.
    skills: [decomposition, code review]
    instruction: |
      Break the goal into cards on the board. ...
    look:
      color: "#D9825B"
      skin: "#E2AE85"
      hair: bun            # short | long | curly | bun | bald
      hair_color: "#2B2420"
      eyes: dots           # dots | glasses | shades | sleepy
      beard: none          # none | stubble | full | mustache
  ilya: { ... }
playbook:
  - scrum_master: Turns the goal into cards on the board
  - lead: Assigns cards. Members branch a worktree each
  - members: Build, then hand off to the lead with tests
  - qa: Challenges every handoff. Failures go back to the author
  - lead: Integrates and opens the PR to main
escalate:
  - when: Blocked for more than 30 minutes
    to: lead
  - when: Board out of date
    to: scrum_master
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `team` | String | yes | | slug; equals the file name | Team id |
| `name` | String | yes | | | Display name |
| `purpose` | String | | | one line | What the team is for, shown under its name on the [Team page](../features/team-page.md). Free text; there are no team kinds |
| `lead` | String | yes | | an id in `agents` | The team lead: the CEO's default contact, the only member who can call `open_pr` |
| `agents` | Map id → [Agent soul](./agent-soul.md) | yes | | at least the lead | Agent ids are **unique across the workspace** (the slug of the display name, which is workspace-unique too), so the instance id `agent@project`, the sandbox path and `wt/<agent>/…` branches can never collide when two teams share a project |
| `playbook` | Array of single-key maps `{who: text}` | | `[]` | | Ordered steps. `who` as in [Playbook and escalation](#playbook-and-escalation) |
| `escalate` | Array of `{when, to}` | | `[]` | `to`: a `who` ([Playbook and escalation](#playbook-and-escalation)) | Escalation table. Project gates ([Project config](./project-config.md)) apply on top |

## Playbook and escalation

A playbook step and an escalation's `to` name **who** with one of:

| `who` | Means | Rendered as |
|---|---|---|
| `lead` | The team lead | The lead's name ("Mara") |
| `members` | Every member except the lead | "Members" |
| `ceo` | The CEO | "You" on the [Team page](../features/team-page.md), `CEO` in the [session prompt](../architecture/session-prompt.md) |
| A role name in snake_case (`scrum_master`, `qa`, `engineer`, …) | The member(s) with that role | Their names |
| An agent id (`tove`) | That agent | Its name |

Anything else (a typo, `scrum master` with a space, a role nobody on the team has) is shown as written and flagged with a warning on the Team page; it is not an error, so a half-edited file still loads.

## Example teams

There are no templates and no team kinds. The [Chief of Staff](../features/chief-of-staff.md) drafts every team definition from the purpose the CEO describes, using a few **example team files shipped with Foreman** as references (read-only, never deployed):

| Example | Purpose | Roster |
|---|---|---|
| Architect | Designs: ADRs, spikes, reviews of designs | Team Lead, Architect, Reviewer, Scrum Master |
| Build | Builds features end to end, tested | Team Lead, Engineer ×2, QA, Scrum Master |
| Challenge | Tries to break what others built | Team Lead, QA ×2, Reviewer |

Each example carries a full roster with job descriptions, a playbook and an escalation table, so the CoS adapts real material rather than filling blanks.

## Invariants

- `lead` names an existing agent of the team.
- Agent display names are unique across the workspace, so @-mentions and avatars stay unambiguous.
- `command` is non-empty when `harness: custom`.
- A team file never carries secrets or deployment state.
- Agent ids are unique across the workspace, not just within the team.
- An agent can't be removed, and a team file can't be deleted, while it is deployed: stop the deployments first. Removing an undeployed agent removes its soul and keeps its sandboxes on disk.

## Writes

The UI, the Chief of Staff and the CEO's editor can all write the same file. Every UI write carries the version it read (a content hash) and is refused on mismatch, like wiki writes. Writes edit the YAML node tree, keeping the CEO's comments and key order, rather than re-marshalling a struct ([Persistence](../architecture/persistence.md#team-file-writes)). Foreman never commits team files: it only writes them, and the CEO versions `.foreman/teams/` themselves if they want to (the Chief of Staff's edits arrive as reviews anyway).
