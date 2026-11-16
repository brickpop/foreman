---
type: epic
status: todo
order: 2
---

# Install and first run

**Goal:** Foreman is one binary that carries everything it needs, and a new install works out of the box: no flag, setting or command to know beforehand, and nothing found by hitting a refusal. Runs side by side with [Messaging and hand-offs](./messaging-handoffs.md) (the CEO's request, 2026-10-16).

Cards: [Release build](../archive/release-build.md) (everything embedded, a foreman-agent per CPU type), [First run](../first-run.md) (sensible defaults, a setup checklist), [New project folder](../archive/new-project-folder.md) (create a project from nothing), [Profiles per agent](../profiles-per-agent.md) and [Model, effort and auto-compact per agent](../agent-model-settings.md) (later; profiles themselves, a harness and its login, one per project, are built: [Session runner → Credentials](../../product/architecture/session-runner.md#credentials)), [Stop containers on exit](../archive/stop-containers-on-exit.md) (agents asleep on the next start) and [Pi as a second harness](../pi-harness.md) (after idle sleep) and [Codex as a harness](../codex-harness.md), and [Free memory before the machine runs out](../memory-pressure.md) (later).

Next (the CEO, 2026-11-15: before installing Foreman on a second machine), in order:
1. [Remove the forge](../archive/remove-forge.md): a clean slate; merges stay local, the CEO pushes code.
2. [One Foreman folder](../foreman-folder.md): `~/foreman/{hq,projects,repos,worktrees}`, HQs, the global repo gone (replaces [Project workspaces](../archive/project-workspaces.md) and its migration).
3. [The wiki at the HQ's root](../archive/wiki-at-root.md).
4. [Portable projects](../portable-projects.md): `projects.toml`, automatic sync of the HQs, restore and Bring here on a new machine. The CEO's one-off move script runs after it, then the new machine restores.

Then [Restructure the New project dialog](../new-project-dialog.md), on the new folders.

**Usable when done:** on a Mac or a Linux machine with nothing but Podman installed (or not even that), the CEO downloads or builds `foreman`, runs it, and is guided to a first deploy; the release binary runs the same on Apple silicon and Intel or AMD machines.

**Later, not in this epic:** CI, published release binaries and Homebrew (Backlog → Not yet scheduled).
