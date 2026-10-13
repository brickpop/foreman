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
kind: build                # architect | build | challenge | custom
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
| `kind` | String | yes | | enum: `architect`, `build`, `challenge`, `custom` | Labels: Architect, Build, Challenge, Custom. Picks the default roster when created by the CoS |
| `lead` | String | yes | | an id in `agents` | The team lead: the CEO's default contact, the only member who can call `open_pr` |
| `agents` | Map id → [Agent soul](./agent-soul.md) | yes | | at least the lead | Agent ids are unique within a team (see open questions) |
| `playbook` | Array of single-key maps `{roleKey: text}` | | `[]` | | Ordered steps. Role keys: `lead`, `members`, `ceo`, or a role (`scrum_master`, `qa`, `architect`, `reviewer`, `engineer`). Rendered with keys resolved to member names, `ceo` → "You" |
| `escalate` | Array of `{when, to}` | | `[]` | `to`: a role key or `ceo` | Escalation table. Project gates ([Project config](./project-config.md)) apply on top |

## Default rosters

Used by the CoS "Create a team" flow.

| Kind | Roster |
|---|---|
| Architect | Team Lead, Architect, Reviewer, Scrum Master |
| Build | Team Lead, Engineer ×2, QA, Scrum Master |
| Challenge | Team Lead, QA ×2, Reviewer |

The first roster entry (Team Lead) becomes `lead`.

## Invariants

- `lead` names an existing agent of the team.
- Agent display names are unique across the workspace, so @-mentions and avatars stay unambiguous.
- `command` is non-empty when `harness: custom`.
- A team file never carries secrets or deployment state.

## Open questions

1. **Agent id collisions across teams.** Agent ids are unique only within a team, but the instance id `agent@project`, the sandbox path `<project>/.sandboxes/<agent>/` and the branch `wt/<agent>/<card>` are per project ([Instance](./instance.md), [Sandboxes and git policy](../architecture/sandboxes-and-git.md)). Two teams deployed on one project with a shared agent id collide on all three. Options: make agent ids workspace-unique (the slug of the already workspace-unique display name); or qualify everything with the team (`agent.team@project`, `.sandboxes/<team>/<agent>/`, `wt/<team>/<agent>/<card>`). Recommendation: workspace-unique ids; it removes the collision everywhere without longer names.
2. **Concurrent edits and comment-preserving writes.** The UI, the CoS and the CEO's editor can write the same YAML at once, and a UI edit must not reformat a hand-edited file. Recommendation: every UI write carries the version it read (content hash) and is refused on mismatch, like wiki writes; writes edit the YAML node tree (keeping comments and key order) rather than re-marshalling a struct.
3. **Versioning team files** (PRD §16.6). If `.foreman/teams` is a git repo, should UI edits commit automatically? Options: no git involvement; auto-commit each save; commit on a debounce. Recommendation: no automatic commits in v1; Foreman only writes files and the CEO versions the folder themselves (the Chief of Staff's edits arrive as reviews anyway).
4. **Playbook role keys.** The PRD uses snake_case (`scrum_master`); the prototype renders `scrum master` and has no `name:` or `look:` keys. The spec follows the PRD; confirm the role-key vocabulary, and whether a key naming an absent role is an error or rendered capitalised (prototype: "Members").
5. **Removing an agent or a team.** Not specified: what happens to running instances, sandboxes and open requests when an agent is removed from a team, or a deployed team's file is deleted. Recommendation: refuse while deployed; otherwise remove the soul and keep sandboxes on disk.
6. **Kind `custom`.** In the PRD but nowhere in the prototype's flows or the Chief of Staff templates. Options: a template that starts with an empty roster except the lead; no template at all, `custom` only for teams written or edited by hand (shown as "Custom" on the [Team page](../features/team-page.md)). Recommendations differ: lead-only roster (from the model) vs hand-written only (from the Team page).
