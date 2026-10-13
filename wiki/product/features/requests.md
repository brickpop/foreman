---
type: spec
status: planned
tags: [feature, requests, acp, runtime]
---

# Requests

Anything that needs the CEO arrives as a typed **request**. Requests are listed on [Home](./home.md) (all projects) and in a project's [Changes](./changes.md) tab, and each new one raises a [notification](./notifications.md). The stored shape is in [Request](../models/request.md); the agent side (`escalate`, `open_pr`) is in [ACP](../architecture/acp.md).

Every request has: kind, source, the instance (who), project, deployment, summary, options (optional), links (optional), created time and status (`open`, `resolved`, `expired`).

## Kinds

| Kind | Raised by | Typical trigger | CEO actions | Blocks the agent? |
|---|---|---|---|---|
| **Approval** | Agent, `escalate(approval)` | A project gate is hit (new dependency, shared infra, scope change vs an accepted ADR) | Pick an option (the first is primary) or type an answer | Yes, until answered |
| **Question** | Agent, `escalate(question)` | The agent needs information only the CEO has | Pick an option or type a free answer | No (the agent may choose to wait) |
| **Review** | Foreman, on the lead's `open_pr()`; or the Chief of Staff for a config change | A PR to `main` is open and checks have run | Merge · Send back with notes · Open session · Open on forge ↗ ([Pipeline and pull requests](./pipeline-and-prs.md)) | No, the lead continues other work |
| **Finding** | Agent, `escalate(finding)` | A challenge team rates an issue high severity | Owning team fixes now · Add to backlog · Not an issue | No |
| **Blocked** | Foreman | Session crashed or exited; harness rate-limited; no output for N minutes (default 20); sandbox or git error; merge conflict on `lead/*` | Retry now · Retry later · Switch harness · Open session | n/a, the instance is stopped |

Request kind chips and colors: [Design system](../design/design-system.md#chips-and-badges).

## Rules

- **Resolving** writes the answer into the agent's session as a marked line (`✓ CEO answered: <answer>. Continuing.`) and marks the request resolved. An approval answer also resumes the paused tool call, so the agent continues from the same point.
- **Deduplication**: a request with the same instance, kind and summary as an open one created within 10 minutes is merged into it.
- **History**: resolved and expired requests are kept in `state.db` ([Persistence](../architecture/persistence.md)).
- **Notification**: every new request triggers a browser notification, subject to [settings](./settings.md).
- **Finding routing**: "Owning team fixes now" sends the finding to the named team's lead as a message in its session (`ⓘ CEO: <team> fixes <finding> now.`); "Add to backlog" asks the deployment's scrum master (or lead) to file a card; "Not an issue" closes it.
- **Blocked actions**: "Retry now" restarts the instance; "Retry later" schedules a restart (30 min); "Switch harness" restarts it with another harness; "Open session" opens it in [Sessions](./sessions.md) ([Agent runtime](../architecture/agent-runtime.md)).
- **Order** in lists: blocked, approvals, reviews, questions, findings; oldest first within a kind.

## Open questions

1. **One definition of "needs you".** The prototype counts differently on Home and the Changes badge (all open requests), in the Chief of Staff answer (approvals + reviews) and on deployment tabs (approvals only). Recommendation: "needs you" = any open request, everywhere (badges, dots, counts).
2. **Free-text answer on approvals.** PRD allows "pick an option or type an answer"; the prototype only offers options. Recommendation: allow a typed answer (useful for "approve, but pin version 2.x").
3. **Should questions block?** (PRD §16.5) PRD: not by default, configurable. Recommendation: non-blocking; `escalate(question)` takes an optional `wait: true` argument for the agent that does want to block ([ACP](../architecture/acp.md)).
4. **Finding routing** (PRD §16.7). Does a finding go straight to the owning team's lead, or only after CEO triage? Recommendation: CEO triage first; the lead is notified only when the CEO picks "<Team> fixes it now".
