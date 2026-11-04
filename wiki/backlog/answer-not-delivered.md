---
type: task
status: todo
priority: high
tags: [bug]
epic: workflow
---

# The CEO's answer to an escalation never reached the lead

**Seen (the CEO, 2026-11-04):** the lead escalated a question (req-172, req-173 while testing), the CEO answered it on the Dashboard, and nothing was typed into the lead's session, not even after the lead finished a later turn. Asked "didn't you get a reply from me?", the lead went looking with `read_message`.

**The path** ([Protocol → `escalate`](../product/architecture/protocol.md#escalate)): `POST /api/requests/{id}/resolve` → `Workflow.Answer` resolves it and `Messenger.Send`s an `answer` message from `ceo` → `Deliver` types it in when the lead is `idle`. It waits when the lead is busy (its next idle brings it), and when the CEO has unsent text in that prompt (the draft countdown). An `asleep` lead is woken first.

**Ruled out so far (reading the code):** the route and the UI call `Answer`; `Queued` selects by recipient and state only; `idle_prompt` and `Stop` both mean idle; hooks after a Foreman restart pass the same authenticated link as tool calls (which worked).

**Still open, needs the CEO's data** (their Foreman runs on the Mac):

- [ ] the message's `state` and `reason` in `team_messages` (`from_instance = 'ceo'`): `queued` means the delivery never ran or found the lead not idle; `delivered` means it was typed and lost in the session
- [ ] the lead's status then (`instances`), and whether Foreman restarted between the escalation and the answer (the lead's session outlived one: [Tool schema after a restart](./tool-schema-after-restart.md))
- [ ] Foreman's log around the answer: `delivering`, `delivering an answer`, `waking for a message`
- [ ] reproduce it with the fake harness, then a test that fails before the fix
