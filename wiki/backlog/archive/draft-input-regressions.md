---
type: task
status: done
priority: high
tags: [bug]
epic: workflow
---

# Preserve Escape and navigation in shared draft tracking

Follow-up to [terminal reply handling](./codex-draft-delivery.md). A standalone Escape is a complete xterm input event, not a prefix for the next keyboard event. Navigation must restart the quiet period, while automatic terminal replies must not.

- [x] Preserve typing, Enter and clear keys after Escape, including characters that could begin a terminal sequence.
- [x] Restore navigation activity without counting reply payload as typing.
- [x] Test both behaviors alongside existing delivery and reply tests; update [Sessions](../../product/features/sessions.md).

- [x] Reduce the default countdown to 20 seconds, retaining the typing quiet period and pause controls (CEO request).

Validation: messaging, server, Claude Code, Pi and Codex tests pass with the race detector; project lint and wiki checks pass. Regression cases cover Escape followed by printable sequence-introducer characters, Enter, Ctrl+C and Ctrl+U; navigation quiet time; and release of paused messages after Escape then clear.
