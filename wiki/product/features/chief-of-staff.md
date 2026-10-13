---
type: spec
status: planned
tags: [feature, ui, agents, teams, config]
---

# Chief of Staff

A built-in agent owned by Foreman: a harness session (Claude Code by default) running in `~/agents-ws/.foreman/cos/`. It interviews the CEO to define roles and teams, edits Foreman files and project config (config on a branch), and handles anything the UI doesn't cover. Its tools are Foreman-specific (read and edit team files, create teams, hire agents, edit `project.yaml` on a branch, raise requests); see [Agent runtime](../architecture/agent-runtime.md) and [Harness adapters](../architecture/harness-adapters.md).

## Drawer

Available from everywhere: the sidebar card, `+ New team`, a team's `Hire`, an agent's `Refine with Chief of Staff`, a project's `Change with Chief of Staff`. A right drawer, `min(420px, 100vw)` ([App shell](./app-shell.md#overlays)).

- **Header**: a 28×28 amber "C" tile (radius 7), "Chief of Staff", the session line `<harness> · ~/agents-ws/.foreman/cos` (Mono 11.5px `--tx4`), and `×`.
- **Messages** (padding 18, gap 14): bubbles (max 88% wide, radius 12, padding `10px 13px`, line-height 1.5, preserved line breaks). The CoS is on the left on `--raised`; the CEO on the right on `--accSoft` with an `--accLine` border.
- **Result card** (full width, `--term`, 1px `--line2`, radius 10, padding `14px 16px`): a title (600) and key/value rows (grid `90px 1fr`, keys `--tx4`, values `--tx2`).
- **Quick-reply chips**, under the last message only. Clicking a chip sends its text as the CEO's answer.
- **Input**: placeholder `Ask for anything…`; Enter sends.
- A typing indicator shows while the CoS is working.
- The conversation history persists; opening the drawer from an entry point starts that flow in the same conversation.
- First open greets: "I set up teams, interview you for new roles, and pick up anything the UI doesn’t cover." with chips `Hire for a role`, `Create a team`, `What needs me today?`.

## Flows

Texts below are the CoS's lines; chips in brackets.

### Hire for a role

From the sidebar card or a team's **Hire**.

1. "Which team is this hire for?" [team names]. Skipped when started from a team: "Hiring for <Team>. What role?"
2. "What role?" [Engineer, QA, Scrum Master, Architect, Reviewer]. Team Lead is not offered.
3. "What will they spend most of their time on? A sentence is enough." Suggestions per role:

   | Role | Suggestions |
   |---|---|
   | Engineer | Backend APIs and migrations · Frontend and UX polish |
   | QA | Property tests and fuzzing · Load and concurrency |
   | Reviewer | Code review against the ADRs · Security review |
   | Architect | Data model and storage · Distributed systems |
   | Scrum Master | Keeping the wiki and board current |
   | other | General work |

4. "How should they work with the rest of the team?" [Heads-down, reports when done · Pushes back, challenges decisions · Collaborative, checks in often]
5. "Which harness should run them?" [Claude Code, Codex CLI, Pi, Hermes Agent]
6. "Here’s the job description. They’ll start on all N of <Team>’s deployments." (or "<Team> isn’t deployed yet, so they’ll start when it is.") A **job description card** titled `<Name> · <Role> · <Team>` with rows `Focus`, `Works`, `Harness`, `Job`. [Hire <Name>, Start over]
   - Job = the role's default instructions plus `Focus: <focus>.`; personality = the working style.
7. On **Hire**: the agent is added to the team file with a random face ([Avatar](../design/avatar.md)) and an instance starts on every deployment of the team. "<Name> has joined <Team>." [Open <Name>’s profile, Hire for a role]

Default instructions per role:

| Role | Instructions |
|---|---|
| Team Lead | Break the goal into cards on the board. / Assign cards and review every handoff before integrating. / You are the only member who opens PRs to main. |
| Engineer | Work only in your own worktree. / Write the test first. Hand off to the lead with a summary and test results. |
| QA | Try to break whatever is handed to you: property tests, fuzzing, races. / File every finding with a repro. Never fix the code yourself. |
| Scrum Master | Own ./wiki. Keep the board in sync with reality. / Record decisions in ADRs. Ping owners of stale cards. |
| Architect | Propose designs as ADRs in wiki/adr/. / Back every claim with a spike and numbers. |
| Reviewer | Review diffs and ADRs for correctness and clarity. / Approve or request changes. Never push code. |

### Create a team

From **`+ New team`**.

1. "What is the new team for?" [Architect it, Build it, Challenge it]
2. "What should we call it?" [suggested names not already used]
3. "Here’s the roster I’d start with. I’ll draft a job description for each role, and you can refine any of them later." A card `<Name> · <Kind>` with one row per role → generated agent name. [Create <Name>, Start over]

   | Template | Roster |
   |---|---|
   | Architect | Team Lead, Architect, Reviewer, Scrum Master |
   | Build | Team Lead, Engineer ×2, QA, Scrum Master |
   | Challenge | Team Lead, QA ×2, Reviewer |

4. On **Create**: writes `.foreman/teams/<id>.yaml` ([Team definition](../models/team-definition.md)) with the lead first, a playbook and escalation rules for the template, and each agent's default instructions. "<Name> is set up. It isn’t deployed anywhere yet." [Deploy to a project, Open <Name>]

### Refine an agent

From the [Agent page](./agent-page.md). "What should change about how <Name> works? This applies on every project they’re deployed to." [Push back more, Communicate more, Narrower focus]. The CoS appends `Adjustment: <text>.` to the job description. "Updated <Name>’s job description. Every running session picks it up on its next turn."

### Change project config

From [Project settings](./project-settings.md). "What should change in <project>/.foreman/project.yaml? I’ll edit it on a branch and send it to you as a review." [Send PRs to GitHub, Add a gate, Change the checks]. The CoS edits the file on branch `foreman/config-N` and raises a **review** request ([Requests](./requests.md)) with the text `Chief of Staff changed <project>/.foreman/project.yaml: <summary>.` and actions Merge / Discard / Open Chief of Staff. "Done: “<summary>”. It’s on branch foreman/config-N and waiting for you in <project> → Changes."

### Anything else

Free text. The CoS acts or answers, and reports back via Home when done or when it needs the CEO. "On it. I’ll work on “<text>” and report back on Home when it’s done or if I need you." Asked "What needs me today?", it summarizes the open requests (same count as Home, [Requests](./requests.md)).

## Open questions

1. **Chat UI on a terminal harness.** The drawer is a structured chat (bubbles, chips, cards), but the CoS is a harness session whose output is a TUI stream. Options: the CoS emits chips and cards through dedicated Foreman tools (e.g. `present_choices`, `show_card`) and its replies through a `say` tool, while the PTY stays available in Sessions for debugging; run the harness headless with structured JSON output. Recommendation: Foreman tools, so it works with any harness.
2. **"New project" flow.** The prototype's CoS creates project folders and asks for git URLs, which contradicts "Foreman never clones". Options: drop it (projects come only from Add a project); allow creating an empty folder, no cloning. Recommendation: drop it in v1.
3. **Hire and create edge cases.** Name collisions with existing agents or teams (display names must be unique across the workspace), an exhausted name pool, an unrecognized team answer (the prototype silently ignores it), and confirming a hire with anything except "Start over". Recommendation: names are checked against the workspace and the CoS proposes another; unrecognized answers get a clarifying question; only an explicit "Hire"/"Create" confirms.
4. **Playbook for a new team.** The prototype copies one fixed playbook regardless of kind. Recommendation: one default playbook and escalation set per template (architect, build, challenge), stored with Foreman's templates.
5. **Custom harness in the interview.** The hire flow doesn't offer Custom. Recommendation: offer it, then ask for the command.
