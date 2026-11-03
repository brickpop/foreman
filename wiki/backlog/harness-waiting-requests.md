---
type: task
status: todo
priority: high
tags: [feature]
epic: workflow
---

# An agent waiting in its own session raises a request

**Why:** an agent can stop at its harness's own question or permission dialog ("Overwrite migrations? 1/2") without calling any tool. Today it only shows as `awaiting`: nothing reaches the Dashboard or the Inbox, so it waits until the CEO happens to open its session. Any agent, lead or member: this is the harness waiting, not an escalation.

Blocked by [Escalate](./escalate.md): it reuses its request answering and delivery.

**Done when:** whenever the harness reports it is waiting for the user, a request (`source: harness`) appears with the question and the choices when the hook carries them, else `<Name> is waiting for your input` and Open session; the CEO's answer is typed into the session; answering in the terminal directly resolves it. Specs: [Protocol → Waiting for the CEO](../product/architecture/protocol.md#waiting-for-the-ceo), [Requests → Rules](../product/features/requests.md#rules).

- [ ] what each harness's hooks carry when it waits (Claude Code's `Notification` with `permission_prompt` and its idle prompt; Pi's): the question text and the choices, or nothing
- [ ] raise on waiting, resolve on the next busy hook (answered in the terminal) or on the CEO's answer; never raised for an agent merely idle after finishing
- [ ] the answer typed through the adapter's `Deliver`: a choice as the key the dialog expects, free text as a line
- [ ] tests: waiting raises one request (not one per repeated hook); idle raises none; answering in the terminal resolves it; the CEO's choice reaches the dialog; a stopped session expires it
