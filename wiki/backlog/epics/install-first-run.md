---
type: epic
status: todo
order: 2
---

# Install and first run

**Goal:** Foreman is one binary that carries everything it needs, and a new workspace works out of the box: no flag, setting or command to know beforehand, and nothing found by hitting a refusal. Runs side by side with [Messaging and hand-offs](./messaging-handoffs.md) (the CEO's request, 2026-10-16).

Cards: [Release build](../release-build.md) (everything embedded, a foreman-agent per CPU type) and [First run](../first-run.md) (sensible defaults, a setup checklist).

**Usable when done:** on a Mac or a Linux machine with nothing but Podman installed (or not even that), the CEO downloads or builds `foreman`, runs it, and is guided to a first deploy; the release binary runs the same on Apple silicon and Intel or AMD machines.

**Later, not in this epic:** CI, published release binaries and Homebrew (Backlog → Not yet scheduled).
