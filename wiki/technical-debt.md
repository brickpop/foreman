---
type: note
tags: [architecture]
---

# Technical debt

Workarounds and accepted limitations to revisit. Each item says why it exists and how it would be fixed. When one is scheduled, it becomes a card in the [backlog](./backlog/index.md) that links back here. When it's fixed, remove it from this page.

## `--demo-session`: one session started by a flag

- **State:** until deployments exist, nothing creates instances. The development flag `foreman --demo-session "<command>"` (used by `just dev`) starts one session, instance `demo`, through the supervisor. The command is split on spaces, with no quoting. The UI's Sessions screen lists whatever sessions exist.
- **Why:** a live session in the browser was needed before deployments, teams and instances exist.
- **Fix:** remove the flag and its wiring in `cmd/foreman` when [deployments](./product/models/deployment.md) start sessions ([Instance manager](./backlog/instance-manager.md)); `just dev` then deploys a test team instead.

## Snapshots: a pending line wrap, and mirrored emulator state

- **State:** terminal snapshots ([Terminal state](./product/architecture/terminal-state.md)) can't restore a pending line wrap: the `charmbracelet/x/vt` emulator keeps it private. `internal/screen/shadow.go` mirrors the pen, scroll region and saved cursor for the same reason. The every-offset round-trip test skips exactly the pending-wrap offsets.
- **Why:** the library exposes no accessors for that state; accepted by the CEO for now.
- **Impact:** a viewer attaching exactly while output sits at the right margin may see one character land a cell off until the program redraws.
- **Fix:** upstream read accessors, then read the state from the emulator and delete `shadow.go`. Tracked in [Snapshots can't restore a pending line wrap](./backlog/snapshot-pending-wrap.md).
