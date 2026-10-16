---
type: task
status: in-progress
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Messaging: ask, reply, read_message, and delivery

**Why:** the lead delegates and teammates answer without the CEO relaying ([Protocol → Team tools](../product/architecture/protocol.md#team-tools), [Protocol → Messages](../product/architecture/protocol.md#messages), [Team tools → Delivery](../product/architecture/team-tools.md#delivery)).

**Done when:** `ask`, `reply` and `read_message` work between the instances of one deployment; messages are stored (`team_messages`, `msg-N` per deployment) and delivered as one plain line when the recipient is idle, a burst as one line; a message to an asleep agent wakes it first; queued messages expire when the deployment stops; markers and the Messages drawer show the traffic.

- [x] the `team_messages` table (migration 003): `ref`, `in_reply_to`, `state`, `reason`, as in [Team message](../product/models/team-message.md); refs never repeat, also under concurrent sends
- [x] the tools (`internal/messaging`), offered exactly as `prompt.Offered` says (a test keeps the two in step), with the protocol's errors; the session prompt describes only the tools offered, plus the outcome rule
- [x] delivery: one plain line typed, then Enter after a short pause; per recipient FIFO; a burst in one line; asleep → `Wake` first; marked delivered before typing (no double delivery), back to queued if typing fails; stopped deployment → `expired`; a sweep at startup; Messaging hears statuses and stops straight from the instance manager
- [x] Restore keeps a live agent's `idle` / `awaiting`, so what is queued for it still goes out after a Foreman restart
- [x] `GET /api/messages?deployment=`, `team.message` events; the UI's messages area is live (markers, the drawer with each message's ref, kind and state)
- [x] the fake harness reports its turns as Claude Code's hooks do and calls tools (`tool ask {…}`), so `just dev fake` runs the real path
- [x] end to end on the real server: Mara asks the asleep Tove; Tove is woken, gets the line when idle, replies; Mara gets hers and reads it in full; Tove can't read Mara's message
- [x] found on the way and fixed: a stopped run's late end could free the run a redeploy started in its place (a flaky redeploy test)
- [ ] the CEO sees Mara and Tove (Claude Code) exchange a message (`just dev`)

Moved to [Git tools](./git-tools.md): `handoff`, which needs `commit` (a member's branch has nothing to hand off without it).
