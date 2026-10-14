---
type: spec
status: planned
tags: [architecture, agents, teams, sessions]
---

# Session prompt

The prompt composer builds each [instance](../models/instance.md)'s session prompt at spawn, from four sources: the [agent soul](../models/agent-soul.md), the [team definition](../models/team-definition.md) (roster, playbook, escalation), the [deployment](../models/deployment.md) (goal) and the [project config](../models/project-config.md) (path, gates, checks). It is passed to the harness as described by its [adapter](./harness-adapters.md).

The [Agent page](../features/agent-page.md) shows this exact composed text, one version per deployment (`GET /api/instances/:id/prompt`, [API](./api.md)). For an agent whose team isn't deployed anywhere, it shows the full template with the deployment-dependent variables (`{project}`, `{path}`, `{goal}`, `{repos}`, `{checks}`, `{gates}`…) left as highlighted placeholders, so the whole shape is visible before deploying.

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
                                    A question doesn't pause you unless you pass wait: true.
open_pr(branch, title, summary)     Propose one of your lead/{team}/* branches for main. Only you can.
                                    Call it again on the same branch to update its PR. (lead only)
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

## Delivery

The composed prompt is plain Markdown, written as the **user-level instruction file of the instance's private harness config directory** ([Session runner → Credentials](./session-runner.md#credentials)):

| Harness | File | Loaded as |
|---|---|---|
| Claude Code | `$CLAUDE_CONFIG_DIR/CLAUDE.md` | User memory, in every session |
| Codex CLI | `$CODEX_HOME/AGENTS.md` | Global instructions |
| Custom | The adapter's choice (a flag with a file path, or the first stdin message) | |

Why this way: nothing is written into the project's repos (an `AGENTS.md` in the sandbox would show up in diffs and PRs), and the project's own `AGENTS.md` / `CLAUDE.md` still apply on top, as project rules. Every harness reads its own file natively, without long command-line flags. It's the same mechanism a human uses to give a harness standing instructions, so it behaves the same way. The file is regenerated at every spawn; a soul edit while running is announced by a context-update message (below) and takes full effect at the next restart. To verify per harness in the [Prove epic](../../backlog/epics/prove.md).

## Variable parts

| Placeholder | Lead | Member |
|---|---|---|
| `{branches}` | `lead/{team}/<topic>, as many as you need (lead/{team}/integration exists). You may open PRs to main, one per branch. When told main moved, rebase that branch onto main and call open_pr again.` | `wt/{agent}/<card> only. No push to main.` |
| `{wiki access}` | `read and write` if the team has no scrum master, else `read only` | `read and write` for the scrum master, else `read only` |
| tools | all five, including `open_pr` | `ask`, `reply`, `handoff`, `escalate` |
| `{ceo section}` | "The CEO talks to you. Break requests down and delegate with ask() or handoff()." | "The CEO normally talks to {lead}. If the CEO instructs you directly, do it, and tell {lead} if it's relevant to the team." |

- `Project: {project}  ({path})` has two spaces before the parenthesis.
- Playbook steps resolve their `who` ([Team definition](../models/team-definition.md#playbook-and-escalation)): `lead` → the lead's name ("Mara"), `members` → "Members", `ceo` → `CEO`, a role name (`scrum_master`) → the name(s) of the member(s) with that role, an agent id → that agent's name. Anything else is shown as written (and flagged on the [Team page](../features/team-page.md)).
- Gates come from `always_ask_ceo`, checks from `checks` in `project.yaml`.
- The branch names follow [Sandboxes and git policy](./sandboxes-and-git.md) (`lead/<team>/*` for the lead, `wt/<agent>/<card>` for members).

## Talking to the CEO

- The default contact for a team is its **lead**. Typing to the lead is a request; the lead delegates via `ask` or `handoff`.
- The CEO can type to any member directly ("direct instruction"): the keystrokes go raw to the member's terminal, under a "Direct line" banner ([Sessions](../features/sessions.md#talking-to-the-team)). Foreman doesn't tell the lead anything; the member's `{ceo section}` (above) asks it to tell the lead if the instruction is relevant to the team.

## Updates

- A soul or team edit doesn't recompose running prompts. Each running instance gets a **context-update message** on its next turn ([Agent runtime](./agent-runtime.md)): the changed sections of its prompt in full (not a diff), prefixed with `ⓘ Your definition changed:`, one message per turn however many saves happened. The full re-prompt happens at the next restart.
- Wiki access follows the PRD: read and write for the scrum master, or for the lead when the team has no scrum master; read only for everyone else (the prototype never granted it to a lead). The container mount mode enforces it ([Sandboxes and git policy](./sandboxes-and-git.md#wiki-access)).
- Roster changes (a hire) are a team edit: existing members learn about the new teammate through the same context update.
