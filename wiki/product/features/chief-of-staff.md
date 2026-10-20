---
type: spec
status: partial
tags: [feature, ui, agents, teams]
---

# Chief of Staff

Foreman's built-in agent for **teams**: it creates a team from what the CEO describes, hires into a team, and refines its members. Nothing else: it raises no requests, appears on no Dashboard, and takes no part in the agents' workflow (the CEO, 2026-10-18). Talking to it is synchronous: the CEO opens its page, talks to it in its terminal, and confirms what it proposes. **It proposes; Foreman writes**, one commit in the [global repo](../models/home.md#the-global-repo), only on the CEO's click.

## How it runs

Decided by the CEO, 2026-10-18 (journey gap G2):

- **An ad-hoc session, not an agent of a deployment**, like the login container of [Connect](./settings.md#profiles): one container with a terminal, started when the CEO opens the [page](#the-page) and stopped when they leave it. It isn't listed in [Sessions](./sessions.md), has no project, team or worktree, and doesn't count towards the instance limit. One at a time: a second tab shows the same session.
- **What it runs:** the `default` profile's harness and login, whatever that is ([Session runner → Credentials](../architecture/session-runner.md#credentials)), started interactively like any agent's, from the agent image.
- **Its conversation carries over:** its private harness config directory is in `~/.foreman/cos/`, so the next visit continues where the last one ended (Claude Code: `--continue`). The opening line is given on every visit, a continued one too (Claude Code: `--continue "<line>"`; to check on a real login).
- **Its id is `cos`**, on the agent port and for its terminal (`/ws/pty/cos`); an agent's is `agent@project`, so they never clash. Its process is named, and labelled with the home (`foreman.home`), like the agents'.
- **Mounts, and nothing else:**
  - `~/.foreman/cos/` → read-write: its working folder and its harness config (outside the global repo);
  - `~/.foreman/config/teams/` → **read-only**: it reads the current team files like any files;
  - the profile's login file, and `foreman-agent` (its tool and hooks), as for any agent.

  Never the rest of the home (secrets, `state.db`, logs, settings, projects, sandboxes), never a code repo, never the CEO's own harness config.
- **Its prompt:** who it is, the team file format and its invariants, the roles' [default job descriptions](../models/agent-soul.md#default-job-descriptions), the names already used in any team, and its one tool. It is composed at each start (`internal/cos`), so the names it lists are those of that moment; the files it reads stay current.

## Its tool

`propose_team(team)`: the whole team definition as it should be ([Team definition](../models/team-definition.md)): a new team, or an existing one with a member added (hire) or changed (refine). One tool for the three cases, since each is the team file as it should become.

- Foreman checks it as it would a save (the [invariants](../models/team-definition.md#invariants): ids and display names unique across all teams, the lead in the roster, valid roles and looks). An invalid proposal is not shown: the call returns the reasons, and the CoS fixes it and calls again.
- A valid one opens the [preview](#the-page) and returns. A newer proposal replaces the one on screen.
- A member proposed without a face (no background color) gets one on the page, drawn once per proposal; **Create** sends those faces, so the one shown is the one saved.
- It never writes. The CEO's **Create** / **Save** does, carrying the version of the team file the proposal was based on: if the file changed meanwhile (an edit on the Agent page, in an editor), nothing is written, the preview says so, and the CoS is told to propose again from the current file.
- Once written, Foreman tells the CoS in its session ("From Foreman: the CEO created Atlas; it is saved.", typed then sent with Enter after a pause, as a delivery is), and the file watcher does the rest (a hired member starts on every deployment of the team, [Deploy a team](./deploy-team.md)).

## The page

`/chief-of-staff` (`?start=create-team`, `?start=hire&team=<id>`, `?start=refine&team=<id>&agent=<id>`), a full page in the main area (a terminal needs the width). Reached from `+ New team` (the sidebar and the setup checklist), a team's **Hire** and an agent's **Refine with Chief of Staff**; it has no sidebar entry of its own (the CEO, 2026-10-18: `+ New team` is it). Each entry point gives the session an opening line, as the goal is a lead's ([Agent runtime → Lazy starts](../architecture/agent-runtime.md#lazy-starts)): `Create a new team.`, `Hire someone for <Team>.`, `Refine <Name> of <Team>.`; the bare URL gives none.

A 1-2-3 flow:

1. **Talk.** The page is the CoS's terminal, full width and height, under a header: the amber "C" tile, "Chief of Staff", the session line `<harness> · default` (Mono 11.5px `--tx4`). The CEO says what they want, in their own words.
2. **Preview.** When the CoS calls `propose_team`, the preview opens on the right, taking most of the width (the terminal stays on the left, narrower, still usable):
   - the team: its name, purpose, playbook and escalation rules;
   - **one tab per member** (avatar and name; the lead first), each showing the role, personality, skills and the full job description. On a hire or refine, the new or changed member is marked and selected, and a changed field shows what it was.
   - **`Adjust team`** closes the preview and puts the focus back in the terminal, where the CEO says what to change; the CoS proposes again. `Show preview` in the header reopens the last one.
3. **Create.** **`Create <Name>`** (a new team) or **`Save <Name>`** (an existing one) writes the file, one commit. A new team opens its [Team page](./team-page.md) (Members); for a changed one the preview closes and the header shows `✓ Saved <Name>`. A refusal (the team changed meanwhile, a newer proposal) shows under the buttons.

Leaving the page stops the session, a second later (coming straight back, or opening it from another entry point, keeps it; closing the browser tab stops it at once); a proposal not yet created is dropped (the CoS still has it in its conversation and can propose it again). The session can't start while Podman isn't ready, nor before the `default` profile is connected: the page says so, with a link to Settings → Agents → Profiles. Under 900px wide, the preview takes the whole width while open.

## Flows

What the CoS asks is in its prompt, not scripted by Foreman; these are the shapes it follows.

- **Create a team:** what the team is for; a name not already used; then the proposal. No templates: it drafts from the purpose, the roles' default job descriptions and the existing teams. Whether Foreman ships starter or example teams for it is still open ([Starter teams](../../backlog/starter-teams.md)).
- **Hire:** which team (given when started from one), the role (not Team Lead), what they'll focus on, how they work with the team; then the team with the new member: the role's default job description plus `Focus: <focus>.`, the working style as personality, a random face ([Avatar](../design/avatar.md)). No harness question: an agent runs on its project's profile ([Agent soul](../models/agent-soul.md#what-it-runs-on)).
- **Refine:** what should change about how the member works; then the team with that member changed (usually `Adjustment: <text>.` appended to the job description).

There is no scrum master role: the lead is the only agent that writes the project's knowledge base. Standups, or nudging the owners of stale cards, can be part of any member's job description.

**Not the CoS's:** projects come only from [Add a project](./add-project.md); `project.toml` is edited in the CEO's editor ([Project config with the Chief of Staff](../../backlog/cos-project-config.md), later).
