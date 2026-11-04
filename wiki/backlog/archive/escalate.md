---
type: task
status: done
priority: high
tags: [feature]
epic: workflow
---

# The lead escalates to the CEO: approvals, questions, findings

**Why:** a lead has no way to reach the CEO but the CEO's own visit to its session: a project gate ("new dependency"), a question only the CEO can answer, or a challenge team's finding waits until the CEO happens to look. Today a request can only be a review ([Request → Phasing](../../product/models/request.md#phasing)).

**Decided (the CEO, 2026-11-04):** `escalate` is the **lead's only**: a member `ask`s its lead, who decides what reaches the CEO. All three kinds in this card, findings and their triage included.

**Done when:** the lead's `escalate(kind, title, body?, options?, wait?)` raises a request on the Dashboard and in the project's Inbox, the CEO answers it there, and the answer reaches the lead's session; specs as built: [Team tools](../../product/architecture/team-tools.md), [Protocol → `escalate`](../../product/architecture/protocol.md#escalate), [Requests](../../product/features/requests.md), [Request](../../product/models/request.md), [Session prompt](../../product/architecture/session-prompt.md).

What exists: the `requests` table and `GET /api/requests` (reviews and stuck-agent cards), the `RequestPanel` with options and a free answer (from the demo), `api.resolve` (stubbed `notBuilt`), the instance's `awaiting` status (from hooks), the `escalate` entry in `prompt.teamTools` (not built, role `anyone`).

- [x] the tool: `prompt.teamTools` entry built and `leadOnly` (members never offered it; `TestOfferedFollowsTheRole`), the router's handler in Workflow: validates `kind` (`bad_kind`) and lengths (`too_long`), saves the request (`source: agent`), deduplicates against open ones (same instance, kind and summary within 10 min, normalized), emits `request.created` (the `⚠` marker waits for markers, built for no tool yet), returns at once with its id and the wait-or-not text
- [x] ~~`awaiting` while an approval is open~~: dropped (the CEO, 2026-11-04): the lead's status doesn't change and messages keep flowing; its rows show it waits on the CEO from its open request (the Session's needs-you dot on its tab)
- [x] answering: `POST /api/requests/{id}/resolve` (answer: an option or free text), resolves and sends the lead an `answer` message from `ceo` (`On your approval "<summary>": <answer>`), delivered like any message; a request whose deployment stopped expires (nothing delivered)
- [x] findings: decided (the CEO, 2026-11-04): a project has one team, so the buttons are the lead's own options and the answer goes back to that lead, like any escalation
- [x] UI: `api.resolve` wired, the panel's options and free answer on the Dashboard and the Inbox, the source label `escalate(<kind>)`
- [x] prompt: the lead's tools line, the messages note's approval sentence (already in `Compose` behind `has("escalate")`); a **member's** gates heading says to take them to its lead (`## Always take to your lead first, for the CEO`), since it has no `escalate`
- [x] tests: a member's call refused (never offered); each kind raises its request; dedup merges and doesn't over-merge (other kind, other instance, after 10 min); answering delivers once, to the right session, only when idle; a second answer to a resolved request refused; an expired request delivers nothing (`internal/workflow/escalate_test.go`, `TestAMemberCantCallTheLeadsTools`)
- [x] title and body (the CEO, 2026-11-04: "This should have a brief title (capped at some point) and optionally a body/description. But not just a single title that must explain everything"): `escalate(kind, title, body?, …)`, a title of one line and at most 120 characters (`teamtools.MaxTitle`, shared with `contribute`; refused `too_long` otherwise, saying to put the rest in `body`), a body of at most 8,000 (migration `018_request_body.sql`, `Request.Body`); dedup compares the body too; a harness's waiting card takes its first line as the title (cut with `…`) and the whole text as the body (`headline`, `TestHeadline`); the panel shows the body as plain text under the title, folded past 6 lines or 480 characters (`RequestBody`, `RequestPanel.test.tsx`)
