---
type: spec
status: planned
tags: [model, teams, agents, config]
---

# Team definition

`~/.foreman/config/teams/<team>.toml`, in the [global repo](./home.md#the-global-repo): a reusable roster of [agent souls](./agent-soul.md) plus the team's playbook and escalation rules. One definition can be [deployed](./deployment.md) to any number of projects; every deployment uses the same souls and the same playbook. Foreman owns these files: the UI ([Team page](../features/team-page.md), [Agent page](../features/agent-page.md)), the [Chief of Staff](../features/chief-of-staff.md) and a deployed team's lead (their [proposals](#proposals), written by Foreman on the CEO's click) and the CEO's editor all write them. A change applies to each instance at its next start; the CEO can [reload](#reload) a deployment to apply it at once.

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
  { lead = "Integrates, then requests the merge" },
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
| `agents` | Map id → [Agent soul](./agent-soul.md) | yes | | at least the lead | Agent ids (the slug of the display name) are **unique within the team only**: two teams may each have a `qa` called QA, a role being the natural name (the CEO, 2026-11-02; [Agents share ids across teams](../../backlog/archive/agents-share-ids-across-teams.md)). One team works on a project at a time, so the instance id `agent@project` never collides; a later team's agent of the same id **takes over** what the earlier one left on the project: its worktree, its `wt/<project>/<agent>/…` branches, its harness folder and logs (the same role in spirit), its conversation starting fresh. Anything naming an agent finds it through the deployment's team (`team@project`), never by the id alone. Their order in the file is the roster's order everywhere, lead first: the Session's agent tabs, Deployment, Members, the session prompt |
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
- Agent display names are unique within the team (compared trimmed), so @-mentions, which only ever name teammates, stay unambiguous.
- A soul names no harness: the project's [profile](../architecture/session-runner.md#credentials) does. An old file's `harness` and `command` lines load, are ignored, and go with the next save ([Agent soul](./agent-soul.md#what-it-runs-on)).
- A team file never carries secrets or deployment state.
- An agent can't be removed, and a team file can't be deleted, while it is deployed: stop the deployments first. Removing an undeployed agent removes its soul and keeps its worktrees and branches on disk.

## Writes

The UI, the Chief of Staff and the CEO's editor can all write the same file. Every UI write carries the version it read (a content hash) and is refused on mismatch, like wiki writes. Each UI save rewrites the whole file and is **one commit** in the global repo, so hand-written comments don't survive a save (the agents keep the file's order, and everything is written flush left), and git keeps every earlier version ([Persistence](../architecture/persistence.md#team-file-writes)). The Chief of Staff and the lead never write the file: they propose, and the CEO's click on the proposal is a UI write like any other, version-guarded against the file it was drafted from ([Proposals](#proposals)).

## Proposals

Two agents can propose a team file, never write it: the [Chief of Staff](../features/chief-of-staff.md#its-tool) (any team, mainly new ones) and, planned, **the lead of a deployed team** (its own team only). That way the CEO can have the team tuned in place, in the conversation that has the context (the CEO, 2026-11-02; [The lead proposes changes to its own team](../../backlog/lead-proposes-team.md)).

- **Same tool, same checks:** `propose_team`, the whole team as it should be, checked like a save ([Invariants](#invariants)). Each proposer has its own proposal on screen; a newer one from the same proposer replaces it.
- **The lead's scope:** its own team's souls (personality, skills, job description), playbook, escalation and purpose, and hiring. Never the team id, removing a member or another team. Members can't propose.
- **Where the CEO sees it:** a lead's proposal opens a pane beside its terminal in the project's [Session](../features/sessions.md), the same pane as the Chief of Staff's, with **Approve** and **Reject**. It shows a **diff**: every changed field, removed text in red, added text in green; unchanged fields aren't highlighted. The Chief of Staff's preview shows the same diff, from the same component.
- **The CEO's approval is the only gate**, so the diff must make every change visible: a lead steered by something it read (a repo, a wiki page) could otherwise slip in a quiet change to an escalation rule or a teammate's job.
- Approved, the file is written (one commit, version-guarded) and the proposer is told. Rejected, the proposer is told and the proposal is dropped.

## Reload

A saved team file reaches an instance at its **next start** (a wake from asleep counts): its prompt is composed at spawn ([Session prompt](../architecture/session-prompt.md)). Nothing restarts on its own. To apply a change at once, the CEO **reloads** the deployment (the CEO, 2026-11-02; built 2026-11-02): each live instance that is behind restarts on the new prompt, **continuing its conversation** (the CEO's Restart, one per agent); asleep ones get it at their wake anyway. Reload is the way to apply a team change to a running team, never Stop and deploy again.

- **Behind:** each start keeps the hash of the prompt it was given ([Instance → `prompt_hash`](./instance.md)); a live instance (`running`, `idle`, `awaiting`) is behind when the prompt composed now differs. So any change that reaches the prompt counts, whoever made it (a proposal, the Agent page, the Chief of Staff, an editor), and so do `project.toml`'s gates and checks, the project's repos and the CEO's instructions; a change that doesn't, doesn't. A run with no hash is behind (the CEO, 2026-11-03: Foreman isn't released, so no run from before the hash needs sparing).
- **Where:** a **`Reload`** button (the reload icon) beside `Stop`, in the project's [Team](../features/project-team.md) header and on each deployment of the [Team page](../features/team-page.md#deployments), **always shown** while the deployment is active: disabled, with the reason, when nothing is behind (`Every agent runs on the current prompt.`) or someone is busy. In the [Session](../features/sessions.md), only while some agent is behind (the CEO, 2026-11-03; built 2026-11-03). One component, `ReloadButton`, its reason in its title. Approving a proposal will also offer it in the pane, as **Reload now?** (planned).
- **Only when nobody is busy**: no agent of the deployment `queued`, `starting`, `running` or `awaiting` (waiting on the CEO may be mid-turn, at a permission prompt), so no turn is ever cut. A line under the header says what it does: `<n> agents run on an older prompt: Reload restarts them on the new one, continuing their conversations.`; while one is busy, the button is disabled and the line says `<n> agents run on an older prompt. <Name> is busy: the change applies at each agent's next start. Reload once they're idle.` (`is waiting on you` for `awaiting`).

**A hire needs no reload** (the CEO, 2026-11-02): when a saved team file has a member with no instance on an active deployment of the team, that member gets one at once, asleep, like any member at deploy. Nothing running is touched. Today it gets none (a bug, whichever way the member was hired): instances are made only at deploy, and the same team can't be deployed again on its project without stopping it first.