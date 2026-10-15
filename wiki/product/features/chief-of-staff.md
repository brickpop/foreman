---
type: spec
status: planned
tags: [feature, ui, agents, teams, config]
---

# Chief of Staff

A built-in agent owned by Foreman: a harness session (Claude Code by default) running in `~/agents-ws/.foreman/cos/`. It interviews the CEO to define roles and teams, changes team files and project config (always on a branch that comes back to the CEO through `request_merge`, as a merge request like any other: Merge to main, Create PR on the forge of the repo it changes, or Reject; [Pipeline and merges](./pipeline-and-merges.md#review-and-merge)), and handles anything the UI doesn't cover. Its tools are Foreman-specific (read team files, create teams, hire agents, refine souls, edit `project.toml`, all on a branch; raise requests); see [Agent runtime](../architecture/agent-runtime.md) and [Harness adapters](../architecture/harness-adapters.md).

## Drawer

Available from everywhere: the sidebar tile, `+ New team`, a team's `Hire`, an agent's `Refine with Chief of Staff`, a project's `Change with Chief of Staff`. A right drawer, `min(420px, 100vw)` ([App shell](./app-shell.md#overlays)).

- **Header**: a 28×28 amber "C" tile (radius 7), "Chief of Staff", the session line `<harness> · ~/agents-ws/.foreman/cos` (Mono 11.5px `--tx4`), and `×`.
- **Messages** (padding 18, gap 14): bubbles (max 88% wide, radius 12, padding `10px 13px`, line-height 1.5, preserved line breaks). The CoS is on the left on `--raised`; the CEO on the right on `--accSoft` with an `--accLine` border.
- **Result panel** (full width, `--term`, 1px `--line2`, radius 10, padding `14px 16px`): a title (600) and key/value rows (grid `90px 1fr`, keys `--tx4`, values `--tx2`).
- **Quick-reply chips**, under the last message only. Clicking a chip sends its text as the CEO's answer.
- **Input**: placeholder `Ask for anything…`; Enter sends.
- A typing indicator shows while the CoS is working.
- The conversation history persists; opening the drawer from an entry point starts that flow in the same conversation.
- First open greets: "I set up teams, interview you for new roles, and pick up anything the UI doesn’t cover." with chips `Hire for a role`, `Create a team`, `What needs me today?`.

## How the drawer is fed

The CoS is a harness session like any other, so its raw output is a TUI stream, not chat bubbles. The drawer is built from **Foreman tools the CoS calls**, which works with any harness:

| Tool | Renders |
|---|---|
| `say(text)` | A CoS bubble |
| `present_choices(choices[])` | Quick-reply chips under the last message |
| `show_panel(title, rows)` | A result panel (job description, roster, config change) |

The CEO's bubbles and chip clicks go to the CoS session as input lines. The CoS's PTY stays visible in [Sessions](./sessions.md) for debugging, like any agent's.

## Flows

Texts below are the CoS's lines; chips in brackets. There is no "New project" flow (the prototype had one that created folders and asked for git URLs, which contradicts "Foreman never clones"): projects come only from [Add a project](./add-project.md).

Rules for every flow:
- **Names** proposed for agents and teams are checked against the workspace (display names are unique across it); on a collision, or when the suggestion pool runs out, the CoS proposes another and lets the CEO type one.
- **An answer the CoS doesn't recognise** (e.g. a team that doesn't exist) gets a clarifying question, never silence.
- **Only an explicit `Hire <Name>` / `Create <Name>`** confirms. Anything else is treated as a change request or a question about the panel.

### Hire for a role

From the sidebar tile or a team's **Hire**.

1. "Which team is this hire for?" [team names]. Skipped when started from a team: "Hiring for <Team>. What role?"
2. "What role?" [Engineer, QA, Architect, Reviewer]. Team Lead is not offered.
3. "What will they spend most of their time on? A sentence is enough." Suggestions per role:

   | Role | Suggestions |
   |---|---|
   | Engineer | Backend APIs and migrations · Frontend and UX polish |
   | QA | Property tests and fuzzing · Load and concurrency |
   | Reviewer | Code review against the ADRs · Security review |
   | Architect | Data model and storage · Distributed systems |
   | other | General work |

4. "How should they work with the rest of the team?" [Heads-down, reports when done · Pushes back, challenges decisions · Collaborative, checks in often]
5. "Which harness should run them?" [Claude Code, Codex CLI, Pi, Hermes Agent, Custom]. Custom is followed by "What command starts it?"
6. "Here’s the job description. They’ll start on all N of <Team>’s deployments." (or "<Team> isn’t deployed yet, so they’ll start when it is.") A **job description panel** titled `<Name> · <Role> · <Team>` with rows `Focus`, `Works`, `Harness`, `Job`. [Hire <Name>, Start over]
   - Job = the role's default instructions plus `Focus: <focus>.`; personality = the working style.
7. On **Hire**: the CoS adds the agent, with a random face ([Avatar](../design/avatar.md)), to the team file on a branch of the [workspace repo](../models/workspace.md#the-workspace-repo) and raises a **review** request through `request_merge`. Once the CEO merges it, an instance starts on every deployment of the team. "<Name> is ready to join <Team>: it’s waiting for you as a review." [Open the review, Hire for a role]

The job is the role's default job description ([Agent soul → Default job descriptions](../models/agent-soul.md#default-job-descriptions), the one source) plus `Focus: <focus>.`

There is no scrum master role: the lead is the only agent that writes the project's knowledge base. Standups, or nudging the owners of stale cards, can be part of any member's job description; such a member proposes board changes to the lead through `contribute`.

### Create a team

From **`+ New team`**. There are no templates: the CoS drafts the team from what the CEO describes, using the example team files shipped with Foreman (Architect, Build, Challenge) as references ([Team definition](../models/team-definition.md#example-teams)).

1. "What is the new team for? A sentence or two is enough." Free text, no fixed chips (e.g. "Build the settlement features, with tests and reviews").
2. "What should we call it?" [suggested names not already used]
3. "Here’s the team I’d start with. I’ll draft a job description for each role, and you can refine any of them later." A panel `<Name>` with the one-line purpose and one row per role → generated agent name, plus the playbook steps. [Create <Name>, Change something, Start over]. "Change something" takes free text and redrafts.
4. On **Create**: writes `.foreman/teams/<id>.toml` ([Team definition](../models/team-definition.md)) with the purpose, the lead first, the drafted playbook and escalation rules, and each agent's drafted job description, on a branch of the [workspace repo](../models/workspace.md#the-workspace-repo), and raises a **review** request through `request_merge`. "<Name> is drafted. Once you merge it, deploy it from its page." [Open the review]

### Refine an agent

From the [Agent page](./agent-page.md). "What should change about how <Name> works? This applies on every project they’re deployed to." [Push back more, Communicate more, Narrower focus]. The CoS appends `Adjustment: <text>.` to the job description on a branch of the workspace repo and raises a **review** request through `request_merge`. "Updated <Name>’s job description on a branch. Once you merge it, every running session picks it up on its next turn."

### Change project config

From [Project settings](./project-settings.md). "What should change in <project>/project/project.toml? I’ll edit it on a branch and send it to you as a review." [Use GitHub as the forge, Add a gate, Change the checks]. The CoS edits the file on branch `foreman/config-N` of the [project repo](../models/project-config.md#the-project-repo) and calls `request_merge`, which raises a **review** request ([Requests](./requests.md)) with the text `Chief of Staff proposes a change to <project>/project/project.toml: <summary>.` and the actions of any review: Merge to main, Create PR (when the project repo has a forge), Reject, plus Open Chief of Staff ([Pipeline and merges](./pipeline-and-merges.md#review-and-merge)). "Done: “<summary>”. It’s on branch foreman/config-N and waiting for you in <project> → Changes."

### Anything else

Free text. The CoS acts or answers, and reports back via Home when done or when it needs the CEO. "On it. I’ll work on “<text>” and report back on Home when it’s done or if I need you." Asked "What needs me today?", it summarizes the open requests (same count as Home, [Requests](./requests.md)).
