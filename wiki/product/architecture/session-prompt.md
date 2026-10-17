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
Your sandbox: {project}/.sandboxes/{agent} (a git worktree per repo)
Branches: {branches}
Knowledge base: {project}/.sandboxes/{agent}/project/wiki ({wiki access})
It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.
Read {project}/.sandboxes/{agent}/project/AGENTS.md first: how this project's wiki and tasks work.

## Your team on {project}
- {name}, {role} (lead: integrates, requests merges to main)
- {name}, {role}
- ...
You can't see their worktrees, only the branches they name. Work with them only through the tools below.
They are separate sessions that Foreman runs: your harness's own agent or messaging tools don't reach them.

## Tools (from Foreman)
ask(teammate, request)              Get info or work. Keep going meanwhile.
reply(request_id, output)           Answer a request. Only `output` is returned.
read_message(id)                    Read a message to you in full (a delivery quotes only its start).
switch(branch)                      Go to one of your branches (made from main if new).
commit(message)                     Commit your changes, in every repo, on your branch.
merge(branch)                       Merge a branch (the lead's, main, a teammate's) into yours.
                                    On conflicts: resolve the markers, then commit().
abort_merge()                       Back out of an unfinished merge.
handoff(teammate, branch, summary)  Pass finished, committed work on. Your part ends. (members only)
contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note.
escalate(kind, summary, options)    Ask the CEO. kind: approval | question | finding.
                                    For an approval, wait for the answer before that step.
request_merge(branch, entry)        Ask the CEO to merge one of your lead/{team}/* branches into main;
                                    entry: the wiki card that explains the work. Only you can.
                                    Call it again on the same branch to refresh it. (lead only)
Git is read-only for you (status, diff, log, blame work); branch, commit
and merge only through switch(), commit() and merge().
Messages arrive in this session as one line, "Message msg-N from <name> (<kind>): …".
Read the whole message with read_message("msg-N"). After a reply, resume the task
that asked for it; after an approval, wait for the CEO's answer, then resume.
A reply (or a handoff) is its outcome: the result, the decisions you took, what is
left open, and how you checked it. A few lines, never a transcript.

## The CEO
{ceo section}

## Playbook
1. {who}: {step}
...

## Always escalate to the CEO
- {gate}
...
Before any handoff, run: {checks}         (the lead: "Before you request a merge, run: {checks}")
```

Without a goal, the line says who gives it: the lead reads `Goal: none set yet: the CEO gives it in this session. Wait for it.`, a member `Goal: none set yet: the CEO gives it to your lead, who tells you what to work on.`

**Phasing:** the prompt describes exactly the team tools Foreman offers the agent (`prompt.Offered`, which the team-tool router serves too, so the two can't drift), and none when the agent started without `foreman-agent`. The [Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md) epic brings them in steps: today `ask`, `reply`, `read_message`, `switch`, `commit`, `merge`, `abort_merge`, `handoff` (members), `contribute` and, since the Git epic (2026-10-17), `request_merge` (the lead); `escalate` is still to come. Each piece of the prompt follows its tool: without `ask`, teammates can't be reached and the CEO relays; without `commit`, a Git section says git is read-only and the CEO reviews the uncommitted work; without `switch`, the agent's branch was made for it; without `request_merge`, the lead's default job doesn't end with merges, and its branches line says to merge accepted hand-offs with `merge` and to tell the CEO which lead branch is ready for review; without `escalate`, the messages note says nothing about approvals. The checks line says "Before any handoff" for a member with `handoff`, "Before you request a merge" for the lead with `request_merge`, else "Before you tell the CEO you're done". The outcome rule (two lines under the tools) comes with `reply`, since an agent otherwise tends to paste a transcript.

## Delivery

The composed prompt is plain Markdown, written as the **user-level instruction file of the instance's private harness config directory** ([Session runner → Credentials](./session-runner.md#credentials)):

| Harness | File | Loaded as |
|---|---|---|
| Claude Code | `$CLAUDE_CONFIG_DIR/CLAUDE.md` | User memory, in every session |
| Codex CLI | `$CODEX_HOME/AGENTS.md` | Global instructions |
| Custom | The adapter's choice (a flag with a file path, or the first stdin message) | |

Why this way: nothing is written into the project's repos (an `AGENTS.md` in the sandbox would show up in diffs and merges), and the project's own `AGENTS.md` / `CLAUDE.md` still apply on top, as project rules. Every harness reads its own file natively, without long command-line flags. It's the same mechanism a human uses to give a harness standing instructions, so it behaves the same way. The file is regenerated at every spawn; a soul edit while running is announced by a context-update message (below) and takes full effect at the next restart. To verify per harness in the [Prove epic](../../backlog/epics/prove.md).

## Variable parts

| Placeholder | Lead | Member |
|---|---|---|
| `{branches}` | `lead/{team}/<topic>, as many as you need (lead/{team}/integration exists). Switch to one (or make it) with switch(branch). Ask for a merge to main with request_merge(branch, entry), one per branch. When told main moved, merge it in with merge("main"), resolve any conflicts and commit(), then call request_merge again.` | `wt/{agent}/<card> only; switch to one (or make it) with switch(branch). No push to main.` |
| `{wiki access}` | `read and write: edit, then commit()` | `read only: use contribute` |
| tools | all but `handoff`, including `request_merge` | all but `request_merge` |
| `{ceo section}` | "The CEO talks to you. Break requests down and delegate with ask() or handoff()." | "The CEO normally talks to {lead}. If the CEO instructs you directly, do it, and tell {lead} if it's relevant to the team." |

- `Project: {project}  ({path})` has two spaces before the parenthesis.
- Playbook steps resolve their `who` ([Team definition](../models/team-definition.md#playbook-and-escalation)): `lead` → the lead's name ("Mara"), `members` → "Members", `ceo` → `CEO`, a role name (`qa`) → the name(s) of the member(s) with that role, an agent id → that agent's name. Anything else is shown as written (and flagged on the [Team page](../features/team-page.md)).
- Gates come from `always_ask_ceo`, checks from `checks` in `project.toml`.
- The branch names follow [Sandboxes and git policy](./sandboxes-and-git.md) (`lead/<team>/*` for the lead; `wt/<agent>/main` and `wt/<agent>/<topic>` for members).

## Talking to the CEO

- The default contact for a team is its **lead**. Typing to the lead is a request; the lead delegates via `ask` or `handoff`.
- The CEO can type to any member directly ("direct instruction"): the keystrokes go raw to the member's terminal, under a "Direct line" banner ([Sessions](../features/sessions.md#talking-to-the-team)). Foreman doesn't tell the lead anything; the member's `{ceo section}` (above) asks it to tell the lead if the instruction is relevant to the team.

## Updates

- A soul or team edit doesn't recompose running prompts. Each running instance gets a **context-update message** on its next turn ([Agent runtime](./agent-runtime.md)): the changed sections of its prompt in full (not a diff), delivered as a message of kind `notice` (`From Foreman: your definition changed. Read it with read_message("msg-N").`), one message per turn however many saves happened. The full re-prompt happens at the next restart.
- Knowledge-base access: read and write for the lead, who commits through `commit()` (no review); read only for everyone else, who use `contribute`. The container mount mode enforces it ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)). `{wiki access}` reads `read and write: edit, then commit()` or `read only: use contribute`.
- Roster changes (a hire) are a team edit: existing members learn about the new teammate through the same context update.
