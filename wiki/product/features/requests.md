---
type: spec
status: planned
tags: [feature, requests, team-tools, runtime]
---

# Requests

Anything that needs the CEO arrives as a typed **request**. Requests are listed on [Home](./home.md) (all projects) and on a project's Overview, under [Needs you](./project-pipeline.md#1-needs-you), and each new one raises a [notification](./notifications.md). The stored shape is in [Request](../models/request.md); the agent side (`escalate`, `request_merge`) is in [Team tools](../architecture/team-tools.md).

Every request has: kind, source, the instance (who), project, deployment, summary, options (optional), links (optional), created time and status (`open`, `resolved`, `expired`).

## Kinds

| Kind | Raised by | Typical trigger | CEO actions | Blocks the agent? |
|---|---|---|---|---|
| **Approval** | Agent, `escalate(approval)`; or the harness itself, waiting at a dialog with choices | A project gate is hit (new dependency, shared infra, scope change vs an accepted ADR); a permission dialog or multiple-choice question in the session | Pick an option (the first is primary) or type an answer | Yes, until answered |
| **Question** | Agent, `escalate(question)`; or the harness itself, waiting for input | The agent needs information only the CEO has; the harness asked something in plain text and stopped | Pick an option or type a free answer | No, unless the agent passes `wait: true` ([Team tools](../architecture/team-tools.md)) |
| **Review** | Foreman, when a [merge request](../models/merge-request.md) is created: on the lead's `request_merge()`, or the Chief of Staff's (project config or a team file) | A branch is ready to merge into `main` of one repo, with a wiki entry explaining it, and checks have run. It is the only request the pipeline raises: handoffs between agents never need the CEO | Merge to main (Foreman merges locally) · Create PR on the forge (only when the repo has one; then Open PR ↗) · Reject with an optional message · Open session (on Home: the card itself) ([Pipeline and merges](./pipeline-and-merges.md)) | No, the lead continues other work |
| **Finding** | Agent, `escalate(finding)` | A challenge team rates an issue high severity | Owning team fixes now · Add to backlog · Not an issue | No |
| **Blocked** | Foreman | Session crashed or exited; harness rate-limited; no output for N minutes (default 20); worktree or git error; integration conflict on `lead/<team>/*` in the lead's worktree | Retry now · Retry later · Switch harness · Open session | n/a, the instance is stopped |

Request kind chips and colors: [Design system](../design/design-system.md#chips-and-badges).

## Rules

- **"Needs you"** means any open request, everywhere: the Home title and sidebar badge, the project's Overview tab badge, project and deployment dots, and the Chief of Staff's "what needs me" answer all count the same set.
- **Answering**: approvals and questions offer their options and a free-text answer field (e.g. "approve, but pin version 2.x"), on Home and on the project's Overview alike: both use the same request panel.
- **Waiting in its session**: whenever the harness reports it is waiting for the user (a permission dialog, a multiple-choice question, or its own "waiting for your input" signal), Foreman raises a request (`source: harness`): an approval with the harness's choices when it exposes them, otherwise a question, carrying the question text when available. The CEO's answer is typed into the session; answering in the terminal directly resolves the request too. The instance is `awaiting` meanwhile. An instance that is merely idle after finishing raises nothing ([Protocol → Waiting for the CEO](../architecture/protocol.md#waiting-for-the-ceo)).
- **Resolving** delivers the answer into the agent's session as one plain line (`From the CEO, on <request>: <answer>.`; for an approval, `… Resume where you paused.`) and marks the request resolved. Nothing is held open: an approval's tool call returned at once ([Team tools → Waiting on approval](../architecture/team-tools.md#waiting-on-approval)).
- **Deduplication**: a request with the same instance, kind and summary as an open one created within 10 minutes is merged into it (summaries compared case- and whitespace-insensitively; see [Request](../models/request.md#invariants)).
- **History**: resolved and expired requests are kept in `state.db` ([Persistence](../architecture/persistence.md)).
- **Notification**: every new request triggers a browser notification, subject to [settings](./settings.md).
- **Finding routing**: a finding goes to the CEO only; the owning team hears about it after the CEO triages it. "Owning team fixes now" sends the finding to the named team's lead as a message in its session (`ⓘ CEO: <team> fixes <finding> now.`); "Add to backlog" asks the deployment's lead to file a card; "Not an issue" closes it.
- **Blocked actions**: "Retry now" restarts the instance; "Retry later" schedules a restart (30 min); "Switch harness" restarts that instance only with another harness, as an override recorded on the [instance](../models/instance.md) until it is next restarted from the team file (the request panel offers to also save it to the soul; see [Agent soul](../models/agent-soul.md)); "Open session" opens it in [Sessions](./sessions.md) ([Agent runtime](../architecture/agent-runtime.md)).
- **Order** in lists: blocked, approvals, reviews, questions, findings; oldest first within a kind.
