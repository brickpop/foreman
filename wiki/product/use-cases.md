---
type: spec
status: planned
tags: [feature, workspace]
---

# Use cases

The end-to-end scenarios v1 must pass. Each one becomes an end-to-end test; the acceptance steps say what must be observable. The cast (projects `ledger`, `relay`; teams Atlas, Vesper; agents Mara, Tove, Ilya, Kestrel, Nyx, Arden) is the prototype's seed data ([prototype](./design/prototype/foreman-prototype.dc.html)).

## 1. Start from scratch

Run `foreman` in `~/agents-ws`, open the UI, then use **+ New project** and add the `ledger` folder (repos already cloned). Next, use **+ New team** to have the Chief of Staff create "Atlas, Build it". Finally, deploy Atlas to ledger with a goal, and the sessions start.

Acceptance:
- [ ] `foreman` started in a folder serves the UI on `localhost:7420`, and `.foreman/` exists in the workspace.
- [ ] The [Add a project](./features/add-project.md) dialog lists `ledger/` with its detected repos; adding it writes `ledger/.foreman/project.yaml` and, if missing, `ledger/wiki/index.md`. Nothing is cloned.
- [ ] The [Chief of Staff](./features/chief-of-staff.md) "Create a team" flow writes `.foreman/teams/atlas.yaml` with the Build roster.
- [ ] [Deploying](./features/deploy-team.md) Atlas to ledger with a goal creates one [deployment](./models/deployment.md), one [instance](./models/instance.md) per member, one sandbox per instance, and every instance reaches `running` or `idle`.

## 2. Delegate through the lead

In Sessions, open the Atlas · ledger tab and type to Mara: "Add idempotency keys to POST /settle". Mara asks Tove, and Tove builds in `wt/tove/LED-44`. Tove hands off to Mara, who integrates. The board card moves.

Acceptance:
- [ ] Text typed in Mara's pane reaches Mara's harness stdin.
- [ ] Mara's `ask(tove, …)` appears as an [ACP message](./models/acp-message.md), in the Messages drawer, and as an ACP marker line in both panes.
- [ ] Tove's commits land on `wt/tove/LED-44` in Tove's sandbox only.
- [ ] Tove's `handoff(mara, wt/tove/LED-44, …)` creates a [pipeline item](./models/pipeline-item.md) at stage 1, visible in [Changes](./features/changes.md) → In flight.
- [ ] When the scrum master (or lead) updates the card's `status` in the wiki, the [Board](./features/board.md) shows it in the new column within 1 s.

## 3. Direct instruction

Open Tove's tab and type a direct instruction. Tove acts, and Mara's session gets a note about it.

Acceptance:
- [ ] Tove's pane shows the "Direct line" banner.
- [ ] The instruction reaches Tove's stdin.
- [ ] Mara's session receives `ⓘ CEO gave Tove a direct instruction: …`.

## 4. Approval gate

Ilya wants a new dependency. An approval request appears on Home and Changes with a notification. The CEO approves and Ilya resumes.

Acceptance:
- [ ] Ilya's `escalate(approval, …)` creates an open approval [request](./models/request.md); Ilya's instance becomes `awaiting`.
- [ ] The request shows on [Home](./features/home.md) and in ledger's Changes; a toast and (if permitted) a browser [notification](./features/notifications.md) fire.
- [ ] Picking "Approve" resolves the request, delivers the answer to Ilya's paused tool call, and Ilya's instance leaves `awaiting`.

## 5. PR review

Mara calls `open_pr`. Foreman opens the PR on Forgejo and checks pass. A review request appears. The CEO merges, or sends it back with notes.

Acceptance:
- [ ] `open_pr` from the lead pushes the lead branch to the forge, opens a PR to `main`, records URL and number on the pipeline item (stage 4), and runs the project checks.
- [ ] A review request appears with the PR link line and Merge / Send back / Open session / Open on forge.
- [ ] Merge merges on the forge, fast-forwards the project repo's `main`, notifies the lead, and lists the item under Merged.
- [ ] Send back posts the notes on the PR, writes them into the lead's session, and resets the item to stage 0.
- [ ] A non-lead calling `open_pr` is refused.

## 6. Challenge team finding

Vesper, deployed on ledger, finds a race. Kestrel escalates a finding, and the CEO picks "Atlas fixes it now", which notifies Mara.

Acceptance:
- [ ] Kestrel's `escalate(finding, …)` creates a finding request on ledger.
- [ ] Choosing "Atlas fixes it now" resolves it and delivers a message into Mara's (Atlas lead on ledger) session.
- [ ] "Add to backlog" and "Not an issue" resolve it without notifying Atlas.

## 7. Reuse a team

Deploy Vesper to relay as well. There are now two independent Vesper deployments with separate sessions and messages. Editing Nyx's soul updates both.

Acceptance:
- [ ] `nyx@ledger` and `nyx@relay` are separate instances with separate sandboxes, logs and ACP channels.
- [ ] An ACP message from `nyx@ledger` can never reach `nyx@relay` or any relay instance.
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
- [ ] The interview ends with a job description card; confirming adds the agent to `atlas.yaml` with a random face.
- [ ] A new instance starts on every active Atlas deployment.

## 10. Change config safely

In Project Settings, choose "Change with Chief of Staff" and ask it to "Send PRs to GitHub". A branch is created with the edit, and a review request appears.

Acceptance:
- [ ] The CoS edits `project.yaml` on a branch `foreman/config-N`, never on `main`.
- [ ] A review request appears in the project's Changes; merging it changes `project.yaml` on `main` and the UI reflects it within 1 s; discarding leaves `main` untouched.

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
- [ ] All instances of that deployment are terminated and marked `stopped`; their sandboxes and branches remain on disk.
- [ ] Deployments of the same team on other projects keep running untouched.
