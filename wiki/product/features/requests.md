---
type: spec
status: partial
tags: [feature, requests, team-tools, runtime]
---

# Requests

Anything that needs the CEO arrives as a typed **request**. Requests are listed in their project's [Inbox](./reviews.md#the-inbox), reviews first, then the others under [Needs you](./project-pipeline.md#1-needs-you) (the CEO, 2026-10-31), and on [Dashboard](./dashboard.md#needs-you) (all projects), but for reviews, which are listed and decided in the Inbox only (the CEO, 2026-10-21); each new one raises a [notification](./notifications.md). The stored shape is in [Request](../models/request.md); the agent side (`escalate`, `request_merge`) is in [Team tools](../architecture/team-tools.md).

Every request has: kind, source, the instance (who), project, deployment, a one-line title (its summary, at most 120 characters), a body (optional: the detail, in Markdown, folded on the panel past 6 lines with **Show all**), options (optional), links (optional), created time and status (`open`, `resolved`, `expired`).

## Kinds

| Kind | Raised by | Typical trigger | CEO actions | Blocks the agent? |
|---|---|---|---|---|
| **Approval** | The lead, `escalate(approval)` | A project gate is hit (new dependency, shared infra, scope change vs an accepted ADR) | Pick an option (the first is primary; Approve · Deny when the lead names none) or type an answer | That step waits for the answer; the lead keeps working on anything else, and gets its messages |
| **Question** | The lead, `escalate(question)`; or the harness itself, waiting for its user (any agent) | The lead needs information only the CEO has; a harness stopped at its own question or permission dialog | An escalation: pick an option or type a free answer. A harness's: Open session, and answer there | An escalation: no, unless the lead passes `wait: true` ([Team tools](../architecture/team-tools.md)). A harness's: the agent waits in its dialog |
| **Review** | Foreman, on the lead's `request_merge()`: one per [review](../models/review.md) of a lead branch, whatever the repos it changed | A branch is ready to merge into a target of each of its repos, with a wiki entry explaining it, and checks have run. It is the only request the pipeline raises: handoffs between agents never need the CEO | In the project's [Inbox](./reviews.md#the-inbox), its page: comments, then Merge (the repos ticked) · Request changes · Dismiss ([Pipeline and merges](./pipeline-and-merges.md#review-and-merge)) | No, the lead continues other work |
| **Finding** | The lead, `escalate(finding)` | The team found an issue the CEO should hear about (a security hole, a race) | The lead's options (e.g. Fix it now · Add to backlog · Not an issue) or a free answer | No |
| **Blocked** | Foreman | Session crashed or exited; harness rate-limited; worktree or git error; integration conflict on `lead/<project>/<team>/*` in the lead's worktree; and, the one built so far, an agent **stuck**: mid-turn with no hook event for N minutes (default 20, [Agent runtime → Stuck agents](../architecture/agent-runtime.md#stuck-agents)) | Retry now · Retry later · Switch profile · Open session; a stuck agent: Restart · Open session | n/a, the instance is stopped; a stuck agent keeps running, and its card resolves itself on its next hook event or when its run ends |

Request kind chips and colors: [Design system](../design/design-system.md#chips-and-badges).

## Rules

- **"Needs you"** means any open request: project and deployment dots, a project's `needs you` state on the Dashboard, and the Chief of Staff's "what needs me" answer all count the same set. The open project's Inbox badge in the sidebar counts them all, reviews included.
- **Answering**: an escalation (approval, question, finding) offers its options and a free-text answer field (e.g. "approve, but pin version 2.x"), on the Dashboard and in the project's Inbox alike: both use the same request panel (`POST /api/requests/:id/resolve`). Answered once: a second answer is refused.
- **Waiting in its session**: whenever the harness reports it is waiting for the user (a permission dialog, a multiple-choice question), Foreman raises a question (`source: harness`) carrying what it asks, and its choices, when the hook says, else `<Name> is waiting for your input.` The CEO answers in the session (Open session); its next activity resolves the request, as does its run ending. Answering from the panel, typed into the dialog, is [not built yet](../../backlog/harness-waiting-requests.md). The instance is `awaiting` meanwhile, and gets no message (typing would answer its dialog). An instance that is merely idle after finishing raises nothing ([Protocol → Waiting for the CEO](../architecture/protocol.md#waiting-for-the-ceo)).
- **Resolving** an escalation marks it resolved and sends the answer to the lead as a message from the CEO, delivered like any other, when its session is idle ([Protocol → `escalate`](../architecture/protocol.md#escalate)). Nothing is held open: the tool call returned at once ([Team tools → Waiting on approval](../architecture/team-tools.md#waiting-on-approval)). While an approval is open the lead keeps its status and its messages; its session tab shows a needs-you dot ([Sessions](./sessions.md)). A deployment that stops expires its open escalations.
- **Dismissing**: every request panel has a × (Dismiss) that closes it unanswered (`POST /api/requests/:id/dismiss`, `status: dismissed`), beside Open session; a review is decided on its page instead. The lead of an escalation is told, so it doesn't wait for an answer that won't come: `On your <kind> "<summary>": the CEO dismissed it, without an answer.` (the CEO, 2026-11-04). A harness card or a Foreman card just closes: the same wait raises no second card, the next one does.
- **Typed answers** are sent with Enter or the **Send** button, and stay in their field until kept: a failure shows as a toast and they can be sent again.
- **Deduplication**: a request with the same instance, kind, title and body as an open one created within 10 minutes is merged into it (compared case- and whitespace-insensitively; see [Request](../models/request.md#invariants)).
- **History**: resolved and expired requests are kept in `state.db` ([Persistence](../architecture/persistence.md)).
- **Notification**: every new request triggers a browser notification, subject to [settings](./settings.md).
- **Findings**: answered like a question, the CEO's pick or text going back to the lead who raised it. One team works on a project at a time, so that lead is who acts on it: no routing to another team (the CEO, 2026-11-04).
- **Blocked actions**: a stuck agent's **Restart** restarts it with the instance's own restart (`POST /api/instances/:id/restart`), and its new run resolves the card; answering other requests isn't built yet. "Retry now" restarts the instance; "Retry later" schedules a restart (30 min); "Switch profile" restarts that agent on another of the CEO's [profiles](../architecture/session-runner.md#credentials) (another account, or another harness), for that agent only (**later**: it needs [Profiles per agent](../../backlog/profiles-per-agent.md); until then the project's profile is changed in its settings, for all its agents); "Open session" opens it in [Sessions](./sessions.md) ([Agent runtime](../architecture/agent-runtime.md)).
- **Order** in lists: blocked, approvals, reviews, questions, findings; oldest first within a kind.
