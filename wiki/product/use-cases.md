---
type: spec
status: planned
tags: [feature, home]
---

# Use cases

The end-to-end scenarios v1 must pass. Each one becomes an end-to-end test; the acceptance steps say what must be observable. The cast (projects `ledger`, `relay`; teams Atlas, Vesper; agents Mara, Tove, Ilya, Kestrel, Nyx, Arden) is the prototype's seed data ([prototype](./design/prototype/foreman-prototype.dc.html)).

## 1. Start from scratch

Run `foreman`, open the UI, then use **+ New project** to make `ledger` and add its repo `~/dev/ledger-api` (already cloned). Next, use **+ New team** to have the Chief of Staff draft "Atlas", a team to build features. Finally, deploy Atlas to ledger with a goal, and the sessions start.

Acceptance:
- [ ] `foreman` serves the UI on `localhost:8888`, and `~/.foreman/config/` exists as a git repo (the global repo). A second `foreman` only prints the first one's URL.
- [ ] The [Add a project](./features/add-project.md) dialog creates the project repo `~/.foreman/projects/ledger/` (`git init`, `project.toml`, a starter wiki) and adds `~/dev/ledger-api` to the project's record by its path. Nothing is cloned, and nothing is written in `~/dev/ledger-api` but its `.git`.
- [ ] The [Chief of Staff](./features/chief-of-staff.md) "Create a team" flow drafts a team from the purpose and shows it in the preview beside its terminal, one tab per member; the CEO's **Create Atlas** writes `~/.foreman/config/teams/atlas.toml`, one commit in the global repo, and Atlas appears in the sidebar within 1 s. A draft that breaks the team file's invariants (a name already used) is never shown.
- [ ] [Deploying](./features/deploy-team.md) Atlas to ledger with a goal creates one [deployment](./models/deployment.md), one [instance](./models/instance.md) per member, one git worktree per instance and code repo (on `lead/atlas/integration` for the lead, `wt/<agent>/main` for members), and every instance reaches `running` or `idle`.

## 2. Delegate through the lead

In Sessions, open the Atlas · ledger tab and type to Mara: "Add idempotency keys to POST /settle". Mara asks Tove, and Tove builds in `wt/tove/LED-44`. Tove hands off to Mara, who integrates. The board card moves.

Acceptance:
- [ ] Text typed in Mara's terminal reaches Mara's harness stdin.
- [ ] Mara's `ask(tove, …)` appears as a [team message](./models/team-message.md), in the Activity panel (who calls whom, and the exchange).
- [ ] Tove's `commit(…)` lands on `wt/tove/LED-44` in Tove's worktree, authored as Tove; no other agent's worktree and not the CEO's checkout change.
- [ ] Inside Tove's container, `git status`, `diff` and `log` work, and `git commit`, `checkout` and `merge` fail (the repo's `.git` is read-only).
- [ ] Tove's `handoff(mara, wt/tove/LED-44, …)` creates a [work item](./models/work-item.md) at stage 1 (no request: handoffs never need the CEO), visible in the project's [Worktrees](./features/worktrees.md) tab, under In flight.
- [ ] Tove's `contribute(finding, …)` writes a note in `~/.foreman/projects/ledger/wiki/inbox/` with Tove as author, and Mara receives `⇄ Tove contributed: …`.
- [ ] When the lead updates the card's `status` in the wiki, the [Board](./features/board.md) shows it in the new column within 1 s.

## 3. Direct instruction

Open Tove's tab and type a direct instruction. Tove acts, and tells Mara if it matters to the team.

Acceptance:
- [ ] Tove's tab shows her role and her branch.
- [ ] Every keystroke reaches Tove's TUI raw, through the same bytes whether typed in the terminal or the line-input box.
- [ ] Foreman posts nothing into Mara's session; Tove's session prompt tells it to inform Mara when the instruction is relevant to the team.

## 4. Approval gate

Ilya wants a new dependency. An approval request appears on the Dashboard and on the project's Overview, with a notification. The CEO approves and Ilya resumes.

Acceptance:
- [ ] Ilya's `escalate(approval, …)` creates an open approval [request](./models/request.md); Ilya's instance becomes `awaiting`.
- [ ] The request shows on [Dashboard](./features/dashboard.md) and on ledger's Overview, under [Needs you](./features/project-pipeline.md#1-needs-you); a toast and (if permitted) a browser [notification](./features/notifications.md) fire.
- [ ] Picking "Approve" resolves the request, delivers the answer to Ilya's paused tool call, and Ilya's instance leaves `awaiting`.

## 5. Merge review

Mara calls `request_merge("lead/atlas/integration", "backlog/LED-44.md")`. Foreman runs the checks. A review request appears. The CEO reviews it in Foreman and merges to main, requests changes with comments, or dismisses it with a reason; or creates a PR on Forgejo and merges there.

Acceptance:
- [ ] `request_merge` from the lead runs the project checks and creates a [merge request](./models/merge-request.md) for each repo with new commits, titled from the entry and carrying every integrated work item (they leave "In flight" and are listed under it). Nothing is pushed to the forge.
- [ ] A review request appears with the entry's title and the repos, and a Review button that opens its review: the files changed by repo, the diff, the checks per repo, and Merge to main / Request changes / Dismiss (Create PR on Forgejo comes with the forge).
- [ ] Comments on lines and general ones stay drafts until Request changes sends them to the lead in one message, each with its repo, file and line; the lead's next `request_merge` shows the new diff, a comment whose line changed is marked outdated, and "Since your last review" shows only what changed.
- [ ] Merge to main merges the branch into `main` on the host, resolves the request, tells the lead, and lists the merge request with its work items under Merged. A branch that doesn't merge cleanly changes nothing, shows "conflicts with main" and asks the lead to merge `main` in.
- [ ] Create PR pushes the branch and opens a PR on the forge; the panel then shows Open PR ↗. When the CEO merges it on the forge, Foreman notices within a polling interval, resolves the request and notifies the lead; closing it without merging rejects the merge request and tells the lead to stop work on it; review comments made on the forge reach the lead's session.
- [ ] On a repo without a forge, Create PR is not offered.
- [ ] Dismiss (the work isn't wanted) keeps the team working (its lead told to stop, with an optional note), or stops it, keeping or dropping the branches (dropped ones recoverable under `refs/foreman/dismissed/`; never under a team left running); closes the PR if there is one; its work items leave In flight.
- [ ] A non-lead calling `request_merge` is refused; so is an entry that doesn't exist in the wiki.
- [ ] Two lead branches (of one lead, or of two teams' leads) can each have an open merge request at the same time; `request_merge` on a branch outside `lead/<own team>/*` is refused.
- [ ] When one merge request is merged, the lead of every other open one on the same repo gets a "main moved" line asking it to merge `main` in with `merge("main")`; after that, `request_merge` on the same branch refreshes the existing merge request (checks rerun, its PR pushed again if it has one) instead of creating a new one.

## 6. Challenge team finding

Vesper, deployed on ledger, finds a race. Kestrel escalates a finding, and the CEO picks "Atlas fixes it now", which notifies Mara.

Acceptance:
- [ ] Kestrel's `escalate(finding, …)` creates a finding request on ledger.
- [ ] Choosing "Atlas fixes it now" resolves it and delivers a message into Mara's (Atlas lead on ledger) session.
- [ ] "Add to backlog" and "Not an issue" resolve it without notifying Atlas.

## 7. Reuse a team

Deploy Vesper to relay as well. There are now two independent Vesper deployments with separate sessions and messages. Editing Nyx's soul updates both.

Acceptance:
- [ ] `nyx@ledger` and `nyx@relay` are separate instances with separate worktrees, logs and team-message channels.
- [ ] A team message from `nyx@ledger` can never reach `nyx@relay` or any relay instance.
- [ ] Editing Nyx's personality writes the team file once, and both running instances receive a context-update message on their next turn.

## 8. Blocked session

Arden's Codex session hits rate limits. Foreman raises a blocked request, and the CEO picks "Switch to Claude Code" or "Retry".

Acceptance:
- [ ] A harness exit or rate-limit marks the instance `stopped` and raises a blocked request with source Foreman.
- [ ] "Retry now" restarts the instance; "Switch to Claude Code" restarts it with the Claude Code harness.
- [ ] A session with no output for the configured time (default 20 min) also raises a blocked request.

## 9. Hire

From Atlas, choose **Hire** and complete the CoS interview. The new engineer joins and starts on every Atlas deployment.

Acceptance:
- [ ] The interview ends with the preview, the new member's tab marked; the CEO's **Save** adds the agent to `atlas.toml`, with a random face, one commit in the global repo.
- [ ] A new instance then starts on every active Atlas deployment.

## 10. Change config safely

*Later* ([Project config with the Chief of Staff](../backlog/cos-project-config.md)): how the change is approved is decided there; the scenario below is the earlier proposal.

In Project Settings, choose "Change with Chief of Staff" and ask it to "Use GitHub as the forge". A branch is created with the edit, and a review request appears.

Acceptance:
- [ ] The CoS edits `project.toml` on a branch `foreman/config-N` of the project repo, never on `main`.
- [ ] A review request appears on the project's Overview, under Needs you, with Merge to main, Create PR (when the project repo has a forge) and Reject; once merged, `project.toml` on `main` changes and the UI reflects it within 1 s; Reject leaves `main` untouched and tells the Chief of Staff why.

## 11. Read the wiki

The Overview opens `index.md`. Open Files, notice the changed-dot on today's standup and read it. Tick a checkbox, which writes the file.

Acceptance:
- [ ] The [Wiki reader](./features/wiki-reader.md) opens `index.md` by default.
- [ ] An entry changed since last viewed shows a dot in the tree and the Files toggle shows a dot while closed; opening the entry clears its dot.
- [ ] Ticking a checkbox changes exactly that one character in the file; a stale version is refused with an error.

## 12. Plan on the board

Drag VSP-12 to "In progress". The frontmatter is updated, and agents see the change through the file.

Acceptance:
- [ ] Dropping the card rewrites only the `status` line (and the lane field when dropped in another lane) of that entry.
- [ ] A write against a stale version is refused and the UI shows the conflict.

## 13. Stop work

On the Team → Deployments tab, choose **Stop** on one project. Its sessions end, worktrees are kept, and other projects continue.

Acceptance:
- [ ] All instances of that deployment are terminated and marked `stopped`; their worktrees and branches remain on disk.
- [ ] Deployments of the same team on other projects keep running untouched.
