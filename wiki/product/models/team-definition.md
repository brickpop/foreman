---
type: spec
status: planned
tags: [model, teams, agents, config]
---

# Team definition

`~/.foreman/config/teams/<team>.toml`, in the [global repo](./home.md#the-global-repo): a reusable roster of [agent souls](./agent-soul.md) plus the team's playbook and escalation rules. One definition can be [deployed](./deployment.md) to any number of projects; every deployment uses the same souls and the same playbook. Foreman owns these files: the UI ([Team page](../features/team-page.md), [Agent page](../features/agent-page.md)), the [Chief of Staff](../features/chief-of-staff.md) (its proposals, written by Foreman on the CEO's click) and the CEO's editor all write them. A change applies to every running instance on its next turn ([Agent runtime](../architecture/agent-runtime.md)).

## Example

```toml
team = "atlas"
name = "Atlas"
purpose = "Builds features end to end, tested and reviewed."   # optional, one line
lead = "mara"
playbook = [
  { lead = "Turns the goal into cards on the board and assigns them. Members work in their own worktrees" },
  { members = "Build, then hand off to the lead with tests" },
  { qa = "Challenges every handoff. Failures go back to the author" },
  { lead = "Integrates, then requests the merge to main" },
]

[[escalate]]
when = "Blocked for more than 30 minutes"
to = "lead"

[agents.mara]
name = "Mara"
role = "team_lead"
personality = "Calm, decisive, terse. Prefers small PRs."
skills = ["decomposition", "code review"]
instruction = """
Break the goal into cards on the board. ...
"""

[agents.mara.look]
color = "#D9825B"
skin = "#E2AE85"
hair = "bun"              # short | long | curly | bun | bald
hair_color = "#2B2420"
eyes = "dots"             # dots | glasses | shades | sleepy
beard = "none"            # none | stubble | full | mustache

[agents.ilya]
# …
```

## Schema

| Field | Type | Required | Default | Constraints | Notes |
|---|---|---|---|---|---|
| `team` | String | yes | | slug; equals the file name | Team id |
| `name` | String | yes | | | Display name |
| `purpose` | String | | | one line | What the team is for, shown under its name on the [Team page](../features/team-page.md). Free text; there are no team kinds |
| `lead` | String | yes | | an id in `agents` | The team lead: the CEO's default contact, the only member who can call `request_merge` |
| `agents` | Map id → [Agent soul](./agent-soul.md) | yes | | at least the lead | Agent ids are **unique across all teams** (the slug of the display name, which is unique across all teams too), so the instance id `agent@project`, the worktree paths and `wt/<agent>/…` branches can never collide when teams follow one another on a project (Stop keeps their worktrees). Their order in the file is the roster's order everywhere, lead first: the Session's agent tabs, Deployment, Members, the session prompt |
| `playbook` | Array of single-key inline tables `{ who = "text" }` | | `[]` | | Ordered steps. `who` as in [Playbook and escalation](#playbook-and-escalation) |
| `escalate` | Array of tables `[[escalate]]` with `when`, `to` | | `[]` | `to`: a `who` ([Playbook and escalation](#playbook-and-escalation)) | Escalation table. Project gates ([Project config](./project-config.md)) apply on top |

## Playbook and escalation

A playbook step and an escalation's `to` name **who** with one of:

| `who` | Means | Rendered as |
|---|---|---|
| `lead` | The team lead | The lead's name ("Mara") |
| `members` | Every member except the lead | "Members" |
| `ceo` | The CEO | "You" on the [Team page](../features/team-page.md), `CEO` in the [session prompt](../architecture/session-prompt.md) |
| A role name in snake_case (`qa`, `engineer`, `reviewer`, …) | The member(s) with that role | Their names |
| An agent id (`tove`) | That agent | Its name |

Anything else (a typo, `team lead` with a space, a role nobody on the team has) is shown as written and flagged with a warning on the Team page; it is not an error, so a half-edited file still loads.

## Example teams

**Open** ([Starter teams](../../backlog/starter-teams.md)): whether Foreman ships example or starter teams at all. There are no templates and no team kinds: the [Chief of Staff](../features/chief-of-staff.md) drafts every team definition from the purpose the CEO describes. The earlier proposal was a few **example team files shipped with Foreman** as its references (read-only, never deployed):

| Example | Purpose | Roster |
|---|---|---|
| Architect | Designs: ADRs, spikes, reviews of designs | Team Lead, Architect, Reviewer |
| Build | Builds features end to end, tested | Team Lead, Engineer ×2, QA |
| Challenge | Tries to break what others built | Team Lead, QA ×2, Reviewer |

Each example carries a full roster with job descriptions, a playbook and an escalation table, so the CoS adapts real material rather than filling blanks.

## Invariants

- `lead` names an existing agent of the team.
- Agent display names are unique across all teams, so @-mentions and avatars stay unambiguous.
- A soul names no harness: the project's [profile](../architecture/session-runner.md#credentials) does. An old file's `harness` and `command` lines load, are ignored, and go with the next save ([Agent soul](./agent-soul.md#what-it-runs-on)).
- A team file never carries secrets or deployment state.
- Agent ids are unique across all teams, not just within the team.
- An agent can't be removed, and a team file can't be deleted, while it is deployed: stop the deployments first. Removing an undeployed agent removes its soul and keeps its worktrees and branches on disk.

## Writes

The UI, the Chief of Staff and the CEO's editor can all write the same file. Every UI write carries the version it read (a content hash) and is refused on mismatch, like wiki writes. Each UI save rewrites the whole file and is **one commit** in the global repo, so hand-written comments don't survive a save (the agents keep the file's order, and everything is written flush left), and git keeps every earlier version ([Persistence](../architecture/persistence.md#team-file-writes)). The Chief of Staff never writes the file: it proposes, and the CEO's click on the proposal is a UI write like any other, version-guarded against the file it was drafted from ([Chief of Staff → Tools](../features/chief-of-staff.md#its-tool)).
