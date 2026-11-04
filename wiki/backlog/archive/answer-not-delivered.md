---
type: task
status: done
priority: high
tags: [bug]
epic: workflow
---

# The CEO's answer to an escalation never reached the lead

**Seen (the CEO, 2026-11-04):** the lead escalated a question (req-172, req-173 while testing), the CEO answered it on the Dashboard, and nothing was typed into the lead's session, not even after the lead finished a later turn. Asked "didn't you get a reply from me?", the lead went looking with `read_message`.

**Root cause** (Foreman's log: `WARN delivering an answer request=req-178 err="… CHECK constraint failed: kind IN ('ask', 'reply', 'handoff', 'contribute', 'notice')"`): `team_messages` refused the `answer` kind, which the spec had but no migration added; and `Workflow.Answer` resolved the request *before* sending, so the failure was only a log line and the card closed as if answered. The workflow tests use a fake messenger, so none touched the real table.

**Fixed (2026-11-04):**

- [x] migration `020_answer_messages.sql` rebuilds `team_messages` with `answer` in its CHECK; `TestEveryMessageKindIsKept` saves every kind through the real store
- [x] `Answer` sends first and resolves only once the message is kept: a failure is an error and the request stays open (`TestTheCEOsAnswerReachesTheLead`)
- [x] reproduced end to end with the fake harness; found on the way a start race: a `SessionStart` hook arriving while the instance was still `starting` was dropped, leaving it `running` with its messages waiting. Hooks are now heard while `starting` (`TestAHookWhileStartingIsKept`)
- [x] requests end once (`Requests.Settle`, open → closed atomically), so dismissing, expiring and an agent's activity can't overwrite each other
