---
type: task
status: done
priority: high
tags: [bug]
epic: workflow
---

# Ignore terminal replies in drafts and allow sending held messages now

Codex can request terminal colors automatically; OSC replies must not count as CEO text or hold messages on an empty prompt. Track escape sequences across input frames. Add a Send now action that expires the draft hold using the existing unfinished-draft delivery semantics, including paused holds, without interrupting a running agent.

[Sessions](../../product/features/sessions.md), [API](../../product/architecture/api.md).

- [x] Ignore terminal control replies, including split OSC/CSI sequences, while preserving actual typing and bracketed paste.
- [x] Add Send now through the messaging interface, HTTP API and banner.
- [x] Test terminal replies and explicit delivery (paused, empty, repeated).
- [x] Update product specs and run tests and wiki checks.

Verification also found two pre-existing platform assumptions: the setup checklist test omitted macOS's machine row, and the housekeeping test expected --no-detach even on Apple Git 2.39, whose maintenance command has no detach option. Correct the assertions to cover the host's behavior; production setup and Git behavior stay unchanged.

Validation: full Go suite and all 304 UI tests pass; release build, lint, wiki health checks, and messaging/server race checks pass.
