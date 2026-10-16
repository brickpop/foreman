---
type: task
status: done
priority: medium
tags: [test]
epic: messaging-handoffs
---

# Claude Code integration tests, then retire cmd/prove

**Why:** experiment D's checks ([Experiment D](./prove-d-claude-delivery-hooks.md)) are what tells us a Claude Code update didn't break delivery or the hooks; they become the adapter's integration tests, and the throwaway experiment code goes.

Unblocked: [Messaging](./messaging-delivery.md) is done.

**Done when:** `just test-claude` (needs the CEO's login and Podman) runs a real Claude Code through Foreman and checks: a fresh session (fresh config, as Foreman writes it) starts with no dialog, a delivered line reaches the prompt and is submitted once, the hooks give `running` → `idle` → `awaiting`, `read_message` and `reply` round-trip; `cmd/prove`, `cmd/proveagent` and their `just` recipes are deleted.

- [x] the test (`cmd/foreman/claude_test.go`), skipped without a login: Foreman's whole stack on a scratch workspace under `.dev/claude-test/` (Podman's machine shares `/Users`, not the system temp folder), the login linked, never copied (a refresh mid-test must reach the real one); passes in about 30 s (2026-10-16)
- [x] delete `cmd/prove`, `cmd/proveagent`, the `prove-*` recipes
- [x] found on the way and fixed: one workspace's Foreman stopped another's agents as orphans (containers now carry their workspace); a stopped container could outlive Foreman (it is now removed before Stop returns); Claude Code writes its transcript a little after the Stop hook (the test waits for it)
