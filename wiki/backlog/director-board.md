---
type: task
status: blocked
priority: medium
tags: [feature, ui]
epic: director
---

# The Director's board

**Why:** the CEO, 2026-11-05. The CEO will run several outcomes at once, some spanning several projects; the Director's conversation alone won't hold them, and the CEO needs to see where each stands. So the Director keeps a board, as a lead keeps its project's.

Blocked by [The Director and the leads](./director-leads.md).

**Decided:**

- **Where:** Foreman's HQ, `~/foreman/hq/outcomes/`: `outcomes/index.md` is the board, one file per outcome. Written by the Director only (its one read-write mount, [The Director as a standing agent](./director-standing.md)), committed with its `commit`.
- **An outcome card:**

  ```markdown
  ---
  type: outcome
  status: active        # active · waiting · done · dropped   (waiting: on the CEO)
  projects: [ledger, relay, docs]
  ---
  # Amounts as decimal strings across the stack

  **Asked:** the CEO, 2026-11-05: "stop using floats for money everywhere"
  **Done when:** every API and client sends amounts as decimal strings; merged on main.

  ## Per project
  | project | state       | lead's card               | last report     |
  |---------|-------------|---------------------------|-----------------|
  | ledger  | in review   | ledger: backlog/led-51.md | 11-05 PR ready  |
  | relay   | not started | —                         | waits on ledger |

  ## Order
  1. ledger first (it defines the API)
  2. relay once ledger is merged

  ## Log
  - 11-05 asked ledger's lead (ledger:msg-12)
  ```

- **The Director keeps it current:** a card when the CEO asks for an outcome, updated on every report, `waiting` when something needs the CEO.
- **The UI:** the Director's page gets a **Board** beside its terminal (the wiki reader and board over Foreman's HQ, the same components as a project's); the Dashboard shows the active outcomes, each opening its card.

**To check first:** whether `wiki check` accepts a link out of the bundle (`../../projects/ledger/hq/backlog/led-51.md`). If not, a lead's card is named as text (`ledger: backlog/led-51.md`), as in the example.

**Steps:**

- [ ] the HQ scaffold's `outcome` type and the board in `wiki.toml` (columns: the statuses)
- [ ] the Director's prompt: the format and the upkeep
- [ ] UI: the Board on the Director's page, active outcomes on the Dashboard
- [ ] specs: the Director's, [Dashboard](../product/features/dashboard.md), [Home](../product/models/home.md) (the HQ's contents)
- [ ] tests: the board read from the HQ; the Dashboard's active outcomes; the Director's commit of a card
