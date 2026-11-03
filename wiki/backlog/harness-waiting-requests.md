---
type: task
status: in-progress
priority: high
tags: [feature]
epic: workflow
---

# An agent waiting in its own session raises a request

**Why:** an agent can stop at its harness's own question or permission dialog ("Overwrite migrations? 1/2") without calling any tool. Today it only shows as `awaiting`: nothing reaches the Dashboard or the Inbox, so it waits until the CEO happens to open its session. Any agent, lead or member: this is the harness waiting, not an escalation.

[Escalate](./archive/escalate.md) is done: its answering is what the dialog step reuses.

**Done when:** whenever the harness reports it is waiting for the user, a request (`source: harness`) appears with the question and the choices when the hook carries them, else `<Name> is waiting for your input` and Open session; the CEO's answer is typed into the session; answering in the terminal directly resolves it. Specs: [Protocol → Waiting for the CEO](../product/architecture/protocol.md#waiting-for-the-ceo), [Requests → Rules](../product/features/requests.md#rules).

- [x] what each harness's hooks carry when it waits (Claude Code's `Notification` with `permission_prompt` and its idle prompt; Pi's): the question text and the choices, or nothing. Built as `Adapter.Asks`: Claude Code's `AskUserQuestion` questions with their labels, its permission prompt's message; Pi's dialog title
- [x] raise on waiting (one card per wait, kind question, no options: answered in the session for now), resolve on the next busy or idle hook (answered in the terminal) or the run's end; a restart keeps the card of an agent still waiting (`internal/instances/waiting.go`); never raised for an agent merely idle after finishing
- [x] tests: waiting raises one request (not one per repeated hook); idle raises none; answering in the terminal resolves it; a stopped session resolves it; a restart keeps it
- [ ] the CEO's choice reaches the dialog: needs a live check of each harness's dialog keys (Claude Code's `AskUserQuestion` and permission dialogs, Pi's `ask_user`) before it's built; then the card offers the hook's choices and `api.resolve` types the pick through the adapter's `Deliver` (a choice as the key the dialog expects, free text as a line), with its test
