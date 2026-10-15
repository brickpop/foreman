---
type: task
status: todo
priority: medium
tags: [test]
epic: messaging-handoffs
---

# Claude Code integration tests, then retire cmd/prove

**Why:** experiment D's checks ([Experiment D](./prove-d-claude-delivery-hooks.md)) are what tells us a Claude Code update didn't break delivery or the hooks; they become the adapter's integration tests, and the throwaway experiment code goes.

Unblocked: [Messaging](./messaging-delivery.md) is done.

**Done when:** `just test-claude` (needs the CEO's login and Podman) runs a real Claude Code through Foreman and checks: a one-line delivery is submitted once, the hooks give `running` → `idle` → `awaiting`, `read_message` and `reply` round-trip; `cmd/prove`, `cmd/proveagent` and their `just` recipes are deleted.

- [ ] the test, skipped without a login
- [ ] delete `cmd/prove`, `cmd/proveagent`, the `prove-*` recipes
