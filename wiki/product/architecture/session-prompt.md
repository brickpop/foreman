---
type: spec
status: planned
tags: [architecture, agents, teams, sessions]
---

# Session prompt

The prompt composer builds each [instance](../models/instance.md)'s session prompt at spawn, from these sources, and last the [CEO's instructions](#the-ceos-instructions): the [agent soul](../models/agent-soul.md), the [team definition](../models/team-definition.md) (roster, playbook, escalation), the [project config](../models/project-config.md) (gates, checks) and the instance's sandbox. It is passed to the harness as described by its [adapter](./harness-adapters.md).

The [Agent page](../features/agent-page.md) shows this exact composed text, one version per deployment (`GET /api/instances/:id/prompt`, [API](./api.md)). For an agent whose team isn't deployed anywhere, it shows the full template with the deployment-dependent variables (`{project}`, `{sandbox}`, `{repos}`, `{checks}`, `{gates}`…) left as highlighted placeholders, so the whole shape is visible before deploying.

## Template

```
# You are {name}, {role} on team {team}
{personality}

## Your job
{instruction}

## This assignment
Project: {project}
Goal: the "Current goal" line at the top of {hq}/backlog/index.md, {who keeps it}
Your sandbox: {sandbox}. A code repo is checked out there, as <sandbox>/<repo>, only once you open it: open(repo), or switch, merge or set_submodule opening what they need. Before working in a repo, open it.
Code repos: {repos}. Read one you haven't opened through its git, no checkout needed: git -C <its path> log {default}, show {default}:<file>, grep -n <text> {default}.
Branches: {branches}
{worktrees persist}
{proving tests}
The project's HQ: {hq} (its one shared checkout, the same for every agent)
Knowledge base: the HQ itself, a wiki at its root, $PROJECT_HQ in your environment ({wiki access})
It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.
{teammates and the wiki}
{reviews in the wiki}
Files from the CEO: {files}/inbox (read only, by design: what the CEO gives the team to work with).
Shared files: {files}/shared (every agent of the project reads and writes it, and it outlasts your tasks and this team: keep there the data later work needs, such as downloads, datasets or generated assets. Knowledge goes in the wiki, code in your branches.)
Read {hq}/AGENTS.md first: how this project's wiki and tasks work.

## Your team on {project}
- {name}, {role} (lead: integrates, requests merges)
- {name}, {role}
- ...
You can't see their worktrees, only the branches they name. Work with them only through the tools below.
They are separate sessions that Foreman runs: your harness's own agent or messaging tools don't reach them.

## Tools (from Foreman)
ask(teammate, request)              Get info or work. Keep going meanwhile.
reply(request_id, output)           Answer a request. Only `output` is returned.
read_message(id)                    Read a message to you in full (a delivery quotes only its start).
open(repo)                          Check a code repo out in your sandbox, the first time you need its
                                    files. switch, merge and set_submodule open what they need.
switch(branch, from)                Go to one of your branches; a new one starts from from, any branch
                                    or commit (each repo's default branch when left out).
commit(commits)                     Commit the repos you name, each with its own message:
                                    [{repo, message}], repo being its folder's name.
merge(branch)                       Merge a branch (the lead's, the CEO's, a teammate's) into yours.
                                    On conflicts: resolve the markers, then commit those repos.
abort_merge()                       Back out of an unfinished merge.
discard(repos)                      Throw away what you didn't commit (all repos if none named).
set_submodule(repo, path, ref, url) Pin a submodule (e.g. lib/forge-std) to a branch, tag or commit;
                                    url adds a new one. Switch and merge keep submodules in step.
handoff(teammate, branch, summary)  Pass finished, committed work on. Your part ends. (members only)
contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note.
escalate(kind, title, body, options)
                                    Ask the CEO. kind: approval | question | finding. A short
                                    one-line title; the detail, if any, in body.
                                    For an approval, wait for the answer before that step. (lead only)
fetch()                             Fetch the repos' remotes: what was pushed upstream shows as
                                    origin/*, to merge in with merge("origin/main"). The CEO's branches don't move. (lead only)
request_merge(branch, title, body, entry, into, land)
                                    Ask the CEO to merge one of your lead/{team}/* branches into into,
                                    a branch of the CEO's (each repo's default when left out). title: what it
                                    brings, one short line; body, if any: what to know to review it;
                                    entry: the wiki card that explains the work. Only you can.
                                    land, if any: a new branch it lands on instead, made from into
                                    when the CEO merges (under {landing prefixes}); the CEO may rename it.
                                    Call it again on the same branch to refresh it. (lead only)
withdraw_merge(branch, reason,      Take back your open request to merge a branch, before the CEO
  superseded_by)                    decides it: superseded_by, another of your branches under review
                                    that holds all of it. When the CEO merges a branch that holds
                                    all of another under review, Foreman withdraws that one itself. (lead only)
reviews()                           Where your reviews stand, with the CEO's comments on changes
                                    requested. Read it again whenever you're unsure. (lead only)
team()                              Where each teammate stands: status, last report, branches.
                                    Check it before you ask a teammate how it's going. (lead only)
Git is read-only for you (status, diff, log, blame work); branch, commit
and merge only through switch(), commit() and merge().
Messages arrive in this session as one line, "Message msg-N from <name> (<kind>): …".
Read the whole message with read_message("msg-N"). After a reply, resume the task
that asked for it; after an approval, wait for the CEO's answer, then resume.
A reply (or a handoff) is its outcome: the result, the decisions you took, what is
left open, and how you checked it. A few lines, never a transcript.
A reply ends its ask: don't answer a reply, and don't ask() just to thank, confirm or
close a loop, since every ask() waits for a reply. Ask only when you need something back.

## The CEO
{ceo section}

## Playbook
1. {who}: {step}
...

## Always escalate to the CEO                 (a member: "Always take to your lead first, for the CEO")
- {gate}
...
Before any handoff, run: {checks}         (the lead: "Before you request a merge, run: {checks}")

## The CEO's standing instructions          (only when set in Settings)
The CEO's own rules for every agent. Where one conflicts with what Foreman says above, Foreman's rules win.

{instructions}
```

**The goal** is the project's, not the deployment's (the CEO, 2026-10-31): the "Current goal" line of the project wiki's `backlog/index.md`, which the starter wiki carries ([Add a project → Effect](../features/add-project.md#effect)). The prompt points to it rather than copying it, since it changes while sessions run. The lead keeps it: it reads it again before planning new work; when it is missing or says none yet, it asks the CEO in its session before anything else and writes it there; when the CEO changes it, it updates it and tells the team. A member reads it again before each new piece of work, and until there is one its lead tells it what to work on.

**Phasing:** the prompt describes exactly the team tools Foreman offers the agent (`prompt.Offered`, which the team-tool router serves too, so the two can't drift), and none when the agent started without `foreman-agent`. The [Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md) epic brings them in steps: today `ask`, `reply`, `read_message`, `switch`, `commit`, `merge`, `abort_merge`, `set_submodule` (since 2026-10-19), `handoff` (members), `contribute` and, since the Git epic (2026-10-17), `request_merge` (the lead), since 2026-10-22 `withdraw_merge` and `reviews` (the lead), since 2026-10-23 `team` (the lead), and since 2026-11-04 `escalate` and `fetch` (the lead). A member has no `escalate`: its gates heading tells it to take them to its lead first. Each piece of the prompt follows its tool: without `ask`, teammates can't be reached and the CEO relays; without `commit`, a Git section says git is read-only and the CEO reviews the uncommitted work; without `open` (no agent link), the sandbox line is `Your sandbox: {sandbox} (your own git worktree of each code repo)`, with no Code repos line: every repo is checked out at its start, idle; without `switch`, its branches line is `none of your own: your worktrees sit on no branch, at {default}.`; without `request_merge`, the lead's default job doesn't end with merges, and its branches line says to merge accepted hand-offs with `merge` and to tell the CEO which lead branch is ready for review; without `escalate`, the messages note says nothing about approvals. **A project with no code repo** (a research team's, the CEO, 2026-11-02) is deployed like any other: its prompt has no `Branches` line, no worktrees and no `{proving tests}`, its sandbox line reads `Your sandbox: {sandbox}, your working folder. This project has no code repo: its work is its knowledge base.`, and it offers none of the git tools ([Team tools → No code repo](./team-tools.md#no-code-repo)), so the lead's tools note says to commit the HQ only through `commit()`, naming `hq`. A built-in role's default job still speaks of code (an engineer's worktree): a research team gives its agents their own. The checks line says "Before any handoff" for a member with `handoff`, "Before you request a merge" for the lead with `request_merge`, else "Before you tell the CEO you're done". The outcome rule (two lines under the tools) comes with `reply`, since an agent otherwise tends to paste a transcript; so does the rule that a reply ends its ask (two more lines), since a lead otherwise answers replies and sends "closing the loop" asks, each of which demands another reply (seen 2026-10-19).

**Working rules** (the CEO, 2026-10-23, from a lead's account of what went wrong): reviews and designs are filed in the wiki and passed on by linking the entry (`{reviews in the wiki}`), since a reviewer's review reached only the lead, who passed on a summary as if it were the full text; the CEO decides from a self-contained brief and agrees the scope before the lead builds what wasn't asked for (`{ceo section}`), since the lead sent the CEO to read documents to decide; few merge requests open, one topic each (`{branches}`), after seven at once; a branch holds one card or topic (`{branches}`), since an audit branch merged into fix branches had them rebuilt twice (there is no cherry-pick: a test is copied in with `git show`); and tests reaching a merge assert correct behaviour only (`{proving tests}`), since proof tests that pass while the bug exists reached a merge request. Since 2026-11-06 the merge-request and branch rules are advice, not rules (the CEO): no cap on merge requests, a card may span several and several may refer to one card, and branches may start from and merge in anything, as long as held-back work is said. Each rule follows the tool it is about, and none goes in `{instruction}`, which a soul's own text replaces.

## The CEO's instructions

`{instructions}` is the text the CEO writes in [Settings → Instructions for every agent](../features/settings.md#instructions-for-every-agent) (kept in [Foreman's HQ](../models/home.md#foremans-hq) as `instructions.md`, so a hand edit counts too), trimmed and as written; with none, the section is left out. Every team agent gets it, lead and members alike, whatever the harness; the [Chief of Staff](../features/chief-of-staff.md) doesn't (the CEO, 2026-10-28). It comes **last**, after Foreman's own rules, and the prompt says Foreman's win on a conflict: a personal note can't override the team tools or the git policy. It is read when the prompt is composed, so a change reaches an agent **at its next start** (no context update), and shows at once on the [Agent page](../features/agent-page.md), whose prompt (and template) includes it.

## Delivery

The composed prompt is plain Markdown, written as the **user-level instruction file of the instance's private harness config directory** ([Session runner → Credentials](./session-runner.md#credentials)):

| Harness | File | Loaded as |
|---|---|---|
| Claude Code | `$CLAUDE_CONFIG_DIR/CLAUDE.md` | User memory, in every session |
| Codex CLI | `$CODEX_HOME/AGENTS.md` | Global instructions |
| Custom | The adapter's choice (a flag with a file path, or the first stdin message) | |

Why this way: nothing is written into the project's repos (an `AGENTS.md` in the sandbox would show up in diffs and merges), and the project's own `AGENTS.md` / `CLAUDE.md` still apply on top, as project rules. Every harness reads its own file natively, without long command-line flags. It's the same mechanism a human uses to give a harness standing instructions, so it behaves the same way. The file is regenerated at every spawn, so a soul edit takes effect at the instance's next start, or at once on a reload ([Updates](#updates)). To verify per harness in the [Prove epic](../../backlog/epics/prove.md).

## Variable parts

| Placeholder | Lead | Member |
|---|---|---|
| `{branches}` | `lead/{team}/<topic>, as many as you need. Switch to one (or make it) with switch(branch, from). Between pieces of work your repos sit on no branch, at their default branch: switch to a topic before you commit or merge (edits made meanwhile come along to a new topic). Ask for a merge with request_merge(branch, title, body, entry, into, land), one per branch, usually into the branch its work started from. To stack them (main < f/a < f/b), request the first with land f/a, and the next into f/a once the CEO merged it. When told the branch it goes into moved, merge that in with merge(<that branch>), resolve any conflicts and commit(), then call request_merge again. Best, a lead branch holds one topic: merge in that topic's hand-offs, and another topic's branch only when the work needs it (its review then shows both). Prefer few merge requests open at once, on topics that don't overlap, each one the CEO can review in one sitting, and see them through before you propose more. When the work justifies more (separate pieces, something urgent), send them; when you hold finished work back instead, tell the CEO what waits. Withdraw a request before you replace it.` (the one-topic sentence only with `merge`). Without `request_merge`: `… Switch to one (or make it) with switch(branch, from). Merge the hand-offs you accept into it with merge(branch). When told a branch your work goes into moved, merge that in with merge(<that branch>). Tell the CEO which lead branch is ready for review.` These are advice, not rules (the CEO, 2026-11-06): a card may span many merge requests, many may refer to the same card, and nothing caps them, but held-back work is said | `wt/{agent}/<card> only; switch to one (or make it) with switch(branch, from). Between pieces of work … (as the lead's). Never push. Best, a card's branch starts from where the lead says its work goes (the default branch unless told) and holds that card's work: merge in another card's branch (an audit's, a proof's) only when the work needs it. To reuse just a test from one, copy it in (git show <branch>:<file>).` (the last two sentences only with `merge`) |
| `{worktrees persist}` (with `switch`; else left out) | `Your worktrees persist between runs: they may hold earlier work. Before you start, check what you have open (ls your sandbox) and where each is (git status, git branch). New work goes on a new topic: switch(<topic>) makes it from the default branch, switch(<topic>, from) from another branch. Earlier work you resume, or commit to its topic before switching. What you don't want: discard(). Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to, and the branch it starts from when it isn't the default.` (the last sentence only with teammates) | the same, without the last sentence. A deployment resumes ([Sandboxes and git policy → Branches](./sandboxes-and-git.md#branches)); what an agent's worktrees hold comes as a notice when its session starts fresh ([Protocol → Deliveries Foreman originates](./protocol.md#deliveries-foreman-originates)) |
| `{wiki access}` | `read and write: edit, then commit its repo, hq` | `read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file` |
| `{teammates and the wiki}` | `Your teammates can't write it: what they add arrives through contribute() in the HQ's inbox/, for you to file. Ask them for content, never to write wiki files or commit hq.` Left out for a lead alone. | (none) |
| `{reviews in the wiki}` (with `contribute`) | `Anything up for review or a decision (a design, a gate review) is filed in the wiki: by its author through contribute(), or by you the moment it arrives. To pass one on, link its entry: never retype it, and never present a summary as the full text.` Left out for a lead alone. | `What you put up for review or a decision (a design, a gate review) goes in through contribute(): then point to the entry it files, never retype it.` |
| `{proving tests}` (with `commit`) | `Tests bound for a merge assert correct behaviour only. A test that passes while a bug exists (a proof, a repro) stays on its own branch, and lands with the fix, as the fix's regression test.` | the same |
| `{landing prefixes}` (in `request_merge`'s row) | `under f/, feat/, fix/`: the project's [`landing_prefixes`](../models/project-config.md); `this project takes none: leave it out` with none | (not offered) |
| tools | all but `handoff`, including `request_merge`, `withdraw_merge` and `reviews` | all but `request_merge`, `withdraw_merge` and `reviews` |
| `{ceo section}` | "The CEO talks to you. Break requests down and delegate with ask()." (the second sentence only with `ask`), then "Agree the scope with the CEO before building anything it didn't ask for." and "When the CEO must decide, bring a brief that stands alone: the question in plain words, who it affects, the options with their consequences, and your recommendation. Documents are optional depth, never required reading." | "The CEO normally talks to {lead}. If the CEO instructs you directly, do it, and tell {lead} if it's relevant to the team." |

- `{hq}` is the project's HQ, `<folder>/projects/<project>/hq`, and `{files}` its files folder beside it, `<folder>/projects/<project>/files` ([Project config → The project's folder](../models/project-config.md#the-projects-folder), [Sandboxes and git policy → Layout](./sandboxes-and-git.md#layout)), both at the same path on the host and in the container: the CEO's files for the team in `inbox/` (read-only), the team's own in `shared/` (the CEO, 2026-11-06). Every project has them, so both lines are always there.
- `{repos}` is each code repo, `<name> at <path> (default <branch>)`, separated by `; `, and `{default}` the first one's default branch, for the examples ([Sandboxes and git policy → Opening a repo](./sandboxes-and-git.md#opening-a-repo)).
- `{sandbox}` is the agent's sandbox, absolute (`<folder>/worktrees/<project>/<agent>`), at the same path on the host and in its container. The prompt names the HQ, the files and the sandbox, never Foreman's folder itself; the code repos live anywhere ([Project config](../models/project-config.md#the-project-record)).
- Playbook steps resolve their `who` ([Team definition](../models/team-definition.md#playbook-and-escalation)): `lead` → the lead's name ("Mara"), `members` → "Members", `ceo` → `CEO`, a role name (`qa`) → the name(s) of the member(s) with that role, an agent id → that agent's name. Anything else is shown as written (and flagged on the [Team page](../features/team-page.md)).
- Gates come from `always_ask_ceo`, checks from `checks` in `project.toml`.
- The branch names follow [Sandboxes and git policy](./sandboxes-and-git.md) (`lead/<project>/<team>/*` for the lead; `wt/<project>/<agent>/main` and `wt/<project>/<agent>/<topic>` for members).

## Talking to the CEO

- The default contact for a team is its **lead**. Typing to the lead is a request; the lead delegates via `ask`.
- The CEO can type to any member directly ("direct instruction"): the keystrokes go raw to the member's terminal, under a "Direct line" banner ([Sessions](../features/sessions.md#talking-to-the-team)). Foreman doesn't tell the lead anything; the member's `{ceo section}` (above) asks it to tell the lead if the instruction is relevant to the team.

## Updates

- A soul or team edit doesn't touch running prompts, and running agents aren't told: each instance gets the new prompt at its next start (a wake counts), or at once when the CEO reloads the deployment ([Team definition → Reload](../models/team-definition.md#reload); the CEO, 2026-11-02, replacing a context-update message that was never built).
- Knowledge-base access: read and write for the lead, who commits it through `commit()`, naming the repo `hq` (no review); read only for everyone else, who use `contribute`. The container mount mode enforces it ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)). `{wiki access}` reads `read and write: edit, then commit its repo, hq` or `read only, by design, not a fault: …`. Both sides are told, because a lead that doesn't know asks members to write wiki files, and a member asked to takes its read-only mount for a fault (seen 2026-10-18: "Arca's sandbox filesystem is read-only"). The `The project's HQ` line calls it the shared checkout for the same reason: a member saw the lead's uncommitted edits there and took it for its own branch.
- Roster changes (a hire) are a team edit: existing members learn about the new teammate the same way, at their next start or on a reload. The lead sees the new member at once through `team()`.
