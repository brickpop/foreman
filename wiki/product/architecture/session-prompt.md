---
type: spec
status: planned
tags: [architecture, agents, teams, sessions]
---

# Session prompt

The prompt composer builds each [instance](../models/instance.md)'s session prompt at spawn, from four sources: the [agent soul](../models/agent-soul.md), the [team definition](../models/team-definition.md) (roster, playbook, escalation), the [deployment](../models/deployment.md) (goal) and the [project config](../models/project-config.md) (path, gates, checks). It is passed to the harness as described by its [adapter](./harness-adapters.md).

The [Agent page](../features/agent-page.md) shows this exact composed text, one version per deployment (`GET /api/instances/:id/prompt`, [API](./api.md)).

## Template

```
# You are {name}, {role} on team {team}
{personality}

## Your job
{instruction}

## This assignment
Project: {project}  ({path})
Goal: {deployment.goal}
Your clone: {project}/.sandboxes/{agent}
Branches: {branches}
Wiki: {project}/wiki ({wiki access})

## Your team on {project}
- {name}, {role} (lead: integrates, opens PRs)
- {name}, {role}
- ...
You can't see their sandboxes. Work with them only through the tools below.

## Tools (from Foreman)
ask(teammate, request)              Get info or work. Keep going meanwhile.
reply(request_id, output)           Answer a request. Only `output` is returned.
handoff(teammate, branch, summary)  Pass finished work on. Your part ends.
escalate(kind, summary, options)    Ask the CEO. kind: approval | question | finding.
open_pr(title, summary)             Propose lead/* for main. Only you can.        (lead only)
Replies arrive in this session as "⇄ <name> replied to req-N".
Resume the task that asked for it. After escalate you resume
from the same point with the CEO's answer.

## The CEO
{ceo section}

## Playbook
1. {who}: {step}
...

## Always escalate to the CEO
- {gate}
...
Before any handoff, run: {checks}
```

## Variable parts

| Placeholder | Lead | Member |
|---|---|---|
| `{branches}` | `lead/*. You may open PRs to main.` | `wt/{agent}/<card> only. No push to main.` |
| `{wiki access}` | `read and write` if the team has no scrum master, else `read only` | `read and write` for the scrum master, else `read only` |
| tools | all five, including `open_pr` | `ask`, `reply`, `handoff`, `escalate` |
| `{ceo section}` | "The CEO talks to you. Break requests down and delegate with ask() or handoff()." | "The CEO normally talks to {lead}. If the CEO instructs you directly, do it and tell {lead} what changed." |

- `Project: {project}  ({path})` has two spaces before the parenthesis.
- Playbook steps resolve role keys to member names (`lead` → "Mara"); `ceo` → `CEO`; unknown keys are capitalised ("Members").
- Gates come from `always_ask_ceo`, checks from `checks` in `project.yaml`.
- The branch names follow [Sandboxes and git policy](./sandboxes-and-git.md) (they change if the lead-branch naming question is resolved with a per-team prefix).

## Talking to the CEO

- The default contact for a team is its **lead**. Typing to the lead is a request; the lead delegates via `ask` or `handoff`.
- The CEO can type to any member directly ("direct instruction"). Foreman:
  1. shows a "Direct line" banner in that session ([Sessions](../features/sessions.md))
  2. posts a system note into the lead's session: `ⓘ CEO gave Tove a direct instruction: …`

## Updates

- A soul or team edit doesn't recompose running prompts. Each running instance gets a **context-update message** on its next turn ([Agent runtime](./agent-runtime.md)); the full re-prompt happens at the next restart.
- Roster changes (a hire) are a team edit: existing members learn about the new teammate through the same context update.

## Open questions

1. **Prompt for an undeployed agent.** The [Agent page](../features/agent-page.md) shows a prompt even when the team isn't deployed (the prototype composes it against an arbitrary project with `Goal: (not deployed)`). Options: show only the soul-only parts with "Deploy the team to see the full prompt"; show the full template with `{project}`-style placeholders and a note that it's filled in at deploy time; show nothing but a note. Recommendations differ: soul-only parts (from this entry) vs placeholder template (from the Agent page).
2. **Context-update wording.** What exactly is injected on a soul edit (a diff, the new full job section, or a short note)? Recommendation: the changed sections in full, prefixed with `ⓘ Your definition changed:`.
3. **Wiki access for the lead.** The PRD gives the lead write access when there is no scrum master; the prototype only ever grants it to the scrum master. Recommendation: follow the PRD (template above).
