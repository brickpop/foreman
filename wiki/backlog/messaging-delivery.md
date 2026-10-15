---
type: task
status: todo
priority: high
tags: [feature]
epic: messaging-handoffs
---

# Messaging: ask, reply, read_message, handoff, and delivery

**Why:** the lead delegates and teammates answer without the CEO relaying ([Protocol → Team tools](../product/architecture/protocol.md#team-tools), [Protocol → Messages](../product/architecture/protocol.md#messages), [Team tools → Delivery](../product/architecture/team-tools.md#delivery)).

Blocked by [Agent link](./agent-link.md).

**Done when:** `ask`, `reply`, `read_message` and `handoff` (its policy check and delivery; checks and work items come with the [Git](./epics/git.md) epic) work between the instances of one deployment; messages are stored (`team_messages`, `msg-N` per deployment) and delivered as one plain line when the recipient is idle, a burst as one line; a message to an asleep agent wakes it first; queued messages expire when the deployment stops; markers and the Messages drawer show the traffic.

- [ ] the `team_messages` table (migration): `ref`, `in_reply_to`, `state`, `reason`, as in [Team message](../product/models/team-message.md)
- [ ] the tools, by role (`handoff` for members only), with the protocol's errors
- [ ] delivery: the adapter's `Deliver` types one line and submits it; per recipient FIFO; a burst in one input; asleep → `Wake` first; stopped deployment → `expired`
- [ ] `GET /api/messages?deployment=`, `team.message` and `instance.marker` events; the UI's messages area goes live (markers, the drawer)
- [ ] tests: routing within a deployment only, one reply per ask, `read_message` only for the addressee, delivery waits for idle, the burst line, waking, expiry

Not here: `contribute` and the git tools ([Git tools](./git-tools.md)).
