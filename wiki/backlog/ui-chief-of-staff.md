---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: full-ui-demo
---

# Chief of Staff drawer

**Why:** the CEO's assistant for everything the screens don't cover ([Chief of Staff](../product/features/chief-of-staff.md)).

Unblocked: the [Demo client](./demo-client.md) and the [App shell](./app-shell.md) are in place.

**Done when:** the drawer (header, chat bubbles, chips on the last message, result panels, input) and its flows on the demo client: hire for a role, create a team, change project config, refine an agent, free text. Built; waiting for the CEO's look on the Mac.

- [x] the drawer and its components (`ChiefOfStaffDrawer`): header with the session line, bubbles, result panels, chips under the last message, the working indicator, the input
- [x] the `chiefOfStaff` client area: `conversation`, `start(entry)`, `send(text)`, and the `cos.changed` event; every entry point starts its flow (sidebar tile, `+ New team`, a team's Hire, an agent's Refine, a project's Change with Chief of Staff, Home and Changes)
- [x] each flow scripted on the demo client (`api/demo/chiefOfStaff.ts`); changes come back as `source: cos` review requests, and merging one applies it (a hire, a new team, a refinement), as the file watcher would
- [x] Chief of Staff items: no project for a team-file change (Home context, notifications), options right on Home, a notification click opens Home
- [x] tests: every flow, unknown answers, name collisions, the exact-confirm rule, merge and reject

Demo choices, not spec changes: at the job panel a single word is taken as the name, anything else as a change to the job; `Open the review` is answered with where the review is.

The server side waits on the [Chief of Staff epic](./epics/chief-of-staff.md) ([technical debt](../technical-debt.md#the-chief-of-staff-is-typed-in-the-ui-only)).
