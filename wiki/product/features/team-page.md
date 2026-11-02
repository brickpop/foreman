---
type: spec
status: planned
tags: [feature, ui, teams]
---

# Team page

One [team definition](../models/team-definition.md): where it is deployed, its members, its playbook and its file. Teams are reusable: one definition, any number of [deployments](../models/deployment.md).

## Header

In Settings, at `/settings/teams/<team>` (the CEO, 2026-11-02: the teams left the sidebar), in its frame: max 760px, padding `24px 24px 48px`, 24px between the header and the tab's content, which have no padding of their own ([Settings → Teams](./settings.md#teams)).

- The breadcrumb `Settings › Teams › <Team>`.
- H1 with the team name; under it, the team's one-line `purpose` (15px `--tx3`), when set ([Team definition](../models/team-definition.md)).
- Subline: `Deployed on N project(s)`, or `Not deployed`.
- Buttons: outline **`Hire`** (the [Chief of Staff](./chief-of-staff.md#the-page), to hire for this team) and primary **`Deploy to a project`** ([Deploy a team](./deploy-team.md), this team preset).
- Tabs (padding `9px 14px`, 500; active `--tx` with a 2px `--acc` bottom border over the 1px `--line` rule, inactive `--tx3`; no badges): `Deployments · Members · Playbook · File`. With no tab in the URL, the page opens on **Deployments** while the team is deployed and on **Members** while it isn't (the CEO, 2026-11-01: nothing to see on an empty Deployments).

## Deployments

One panel per deployment (padding `16px 20px`):

- The whole panel opens the project's [Session](./sessions.md) (the CEO, 2026-10-16: from a team, the CEO wants that team at work). Inside it: the project name (600 15px), which alone opens the project's [Wiki](./wiki-reader.md); and `N working` plus ` · M waiting on you` when the deployment has open requests ([Requests](./requests.md)). That status line is the sessions link, stretched over the whole panel, so each target is a real link for the keyboard; the panel shows its focus ring.
- A **`Stop`** button in the danger style ([Design system](../design/design-system.md#buttons)), led by the stop icon, a filled square; so is the confirmation's `Stop`. The word stays: the sessions do end, and "Unassign" would hide that (the CEO, 2026-10-21).
- **Stop** (also in the project's [Deployment](./project-deployment.md) section) ends that deployment only: its sessions terminate; worktrees and branches stay on disk; other deployments of the team keep running ([Agent runtime](../architecture/agent-runtime.md)). Stop first asks for confirmation in a dialog: "Stop <Team> on <project>? <N> sessions end. Worktrees and branches are kept." with `Cancel` and a danger `Stop`. Then a toast confirms. Its open requests expire and its work items and pull requests stay visible as stopped ([Deployment](../models/deployment.md#lifecycle)).
- **Empty**: `Not deployed anywhere. Deploy it to a project to start sessions.`

## Members

One row per agent (grid `1fr auto`, padding `14px 6px`), lead first; a click opens the [Agent page](./agent-page.md).

- `<Name> · <Role>` (the suffix 400 `--tx4`; no harness: the project's profile says what it runs on), then the personality (13.5px `--tx3`, ellipsis).
- Right: for each deployment, a 7px status dot and the project's readable name ([Instance](../models/instance.md)).

## Playbook

- Intro: "The playbook is part of the team definition, so every deployment uses the same one. It's added to each member's session prompt. Each project's own gates and checks apply on top."
- Steps: grid `28px 110px 1fr`: the number (Mono `--tx4`), who (600), the text. `who` resolves as in [Playbook and escalation](../models/team-definition.md#playbook-and-escalation): `lead` → the lead's name ("Mara"), `members` → "Members", `ceo` → "You", a role → its members' names, an agent id → its name. An unknown `who` is shown as written with a warning icon (`--attn`) and the tooltip "No member matches this; check the team file".
- "Escalate": one line per rule, the condition (`--tx2`) and the target (500); `ceo` shows as "You" in `--attnTx`.

## File

The path `<home>/config/teams/<id>.toml` (the home's real path, e.g. `~/.foreman/config/teams/atlas.toml`, in the [global repo](../models/home.md#the-global-repo)) and the file's actual content in a read-only `<pre>`, exactly as on disk ([Team definition](../models/team-definition.md)).
