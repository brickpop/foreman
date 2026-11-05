---
type: task
status: todo
priority: low
tags: [bug, decision]
epic: workflow
---

# An agent keeps an old tool schema across a Foreman restart

**Seen (2026-11-04):** after Foreman was rebuilt with `escalate(kind, title, body, …)`, a lead whose session outlived the restart still had the old schema (`summary`): its call was refused (`empty: say what you need from the CEO in title`), and it only went through when the agent tried fields its schema didn't list. Its harness loaded the tool list when its session started and kept it.

**Why it matters:** every change to a tool's arguments breaks every running session until it restarts, silently, and agents will report it as a Foreman bug (this one did). It will come back with every Foreman upgrade, not only in development.

**To decide (the CEO):**

- tell the sessions their tools changed (MCP's `notifications/tools/list_changed`), if each harness takes it mid-session (to check, per harness)
- or restart (continuing the conversation) the sessions started under an older Foreman, once idle
- or accept it, and say so on the agents' refusals

**Built first (2026-11-05, the CEO's pick of the cheapest):** a call with an argument the tool doesn't take is refused naming the current ones (`unknown_arguments`, [Team tools → Per-instance setup](../product/architecture/team-tools.md#per-instance-setup); `TestArgumentsTheToolDoesntTakeAreRefused`), so the agent recovers on its own call. Not covered: a field renamed whose old name is simply left out, with nothing extra sent, still gets the block's own refusal (which names the field it wants, so it recovers too); a changed meaning with the same names. The two options above stay open, lower priority now (medium → low).
