---
type: epic
status: todo
order: 2
---

# Install and first run

**Goal:** Foreman is one binary that carries everything it needs, and a new install works out of the box: no flag, setting or command to know beforehand, and nothing found by hitting a refusal. Runs side by side with [Messaging and hand-offs](./messaging-handoffs.md) (the CEO's request, 2026-10-16).

Cards: [Release build](../archive/release-build.md) (everything embedded, a foreman-agent per CPU type), [First run](../first-run.md) (sensible defaults, a setup checklist), [New project folder](../archive/new-project-folder.md) (create a project from nothing), [Profiles per agent](../profiles-per-agent.md) and [Model, effort and auto-compact per agent](../agent-model-settings.md) (later; profiles themselves, a harness and its login, one per project, are built: [Session runner → Credentials](../../product/architecture/session-runner.md#credentials)), [Stop containers on exit](../archive/stop-containers-on-exit.md) (agents asleep on the next start) and [Pi as a second harness](../pi-harness.md) (after idle sleep) and [Codex as a harness](../codex-harness.md).

Next (the CEO, 2026-10-28): [Project workspaces](../project-workspaces.md) (a project's files in a folder of the CEO's, out of `~/.foreman`), then [Restructure the New project dialog](../new-project-dialog.md) on top of it.

**Usable when done:** on a Mac or a Linux machine with nothing but Podman installed (or not even that), the CEO downloads or builds `foreman`, runs it, and is guided to a first deploy; the release binary runs the same on Apple silicon and Intel or AMD machines.

**Later, not in this epic:** CI, published release binaries and Homebrew (Backlog → Not yet scheduled).
