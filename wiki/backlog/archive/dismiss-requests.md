---
type: task
status: done
priority: high
tags: [feature]
epic: workflow
---

# Dismiss a request from the Inbox

**Asked (the CEO, 2026-11-04):** "in addition to opening the session, I would expect a way to dismiss an alert/finding/etc." Decided: an escalation's lead is told, a harness or Foreman card just closes ([Requests → Rules](../../product/features/requests.md#rules)).

- [x] `status: dismissed`; `Workflow.Dismiss` and `POST /api/requests/:id/dismiss` (409 when closed already, 400 for a review)
- [x] the lead of a live deployment gets an `answer` message: `On your <kind> "<summary>": the CEO dismissed it, without an answer.`
- [x] a dismissed harness card isn't raised again for the same wait, nor overwritten by its end
- [x] the request panel's × (Dashboard and Inbox alike); the typed answer gets a **Send** button and stays in its field until kept
