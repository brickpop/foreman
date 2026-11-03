---
type: task
status: todo
priority: high
tags: [feature]
epic: workflow
---

# The lead escalates to the CEO: approvals, questions, findings

**Why:** a lead has no way to reach the CEO but the CEO's own visit to its session: a project gate ("new dependency"), a question only the CEO can answer, or a challenge team's finding waits until the CEO happens to look. Today a request can only be a review ([Request → Phasing](../product/models/request.md#phasing)).

**Decided (the CEO, 2026-11-04):** `escalate` is the **lead's only**: a member `ask`s its lead, who decides what reaches the CEO. All three kinds in this card, findings and their triage included.

**Done when:** the lead's `escalate(kind, summary, options?, wait?)` raises a request on the Dashboard and in the project's Inbox, the CEO answers it there, and the answer reaches the lead's session; specs as built: [Team tools](../product/architecture/team-tools.md), [Protocol → `escalate`](../product/architecture/protocol.md#escalate), [Requests](../product/features/requests.md), [Request](../product/models/request.md), [Session prompt](../product/architecture/session-prompt.md).

What exists: the `requests` table and `GET /api/requests` (reviews and stuck-agent cards), the `RequestPanel` with options and a free answer (from the demo), `api.resolve` (stubbed `notBuilt`), the instance's `awaiting` status (from hooks), the `escalate` entry in `prompt.teamTools` (not built, role `anyone`).

- [ ] the tool: `prompt.teamTools` entry built and `leadOnly` (members never offered it; `TestOfferedFollowsTheRole`), the router's handler in Workflow: validates `kind` (`bad_kind`) and lengths (`too_long`), saves the request (`source: agent`), deduplicates against open ones (same instance, kind and summary within 10 min, normalized), emits `request.created` and the `⚠` marker, returns at once with its id and the wait-or-not text
- [ ] `awaiting`: the lead is `awaiting` while an approval (or a `wait: true` question) of its is open, and leaves it when answered, together with what hooks say
- [ ] answering: `POST /api/requests/{id}/resolve` (answer: an option or free text), resolves and delivers `From the CEO, on <request>: <answer>.` (`… Resume where you paused.` for a waited one) through `Instances.Deliver`; a request whose deployment stopped expires (nothing delivered)
- [ ] findings: the CEO's three actions. "<Team> fixes it now" delivers the finding to that team's lead on the project; "Add to backlog" asks the raising lead to file a card; "Not an issue" closes it. Which teams "<Team> fixes it now" offers: the project's deployed teams (to confirm while building)
- [ ] UI: `api.resolve` wired, the panel's options and free answer on the Dashboard and the Inbox, the source label `escalate(<kind>)`
- [ ] prompt: the lead's tools line, the messages note's approval sentence (already in `Compose` behind `has("escalate")`); a **member's** gates heading says to take them to its lead (`## Always take to your lead first, for the CEO`, wording to confirm), since it has no `escalate`
- [ ] tests: a member's call refused (never offered); each kind raises its request; dedup merges and doesn't over-merge (other kind, other instance, after 10 min); answering delivers once, to the right session, only when idle; a second answer to a resolved request refused; an approval holds `awaiting` until answered; an expired request delivers nothing; the finding actions reach the right lead and only it
