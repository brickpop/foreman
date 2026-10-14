---
type: note
tags: [architecture]
---

# Technical debt

Workarounds and accepted limitations to revisit. Each item says why it exists and how it would be fixed. When one is scheduled, it becomes a card in the [backlog](./backlog/index.md) that links back here. When it's fixed, remove it from this page.

## `--demo-session`: one session started by a flag

- **State:** until deployments exist, nothing creates instances. The development flag `foreman --demo-session "<command>"` (used by `just dev`) starts one session, instance `demo`, through a start-by-instance map in `cmd/foreman/sessions.go`. The command is split on spaces, with no quoting. The UI's Sessions screen lists whatever sessions exist.
- **Why:** a live session in the browser was needed before deployments, teams and instances exist.
- **Fix:** deployments start sessions now ([Instance manager](./backlog/instance-manager.md)), but nothing in the UI deploys a team yet. Remove the flag and its wiring in `cmd/foreman` (`sessions.go`, the demo part of `views.go`) once the Deploy dialog is wired to the real server ([Teams and agents](./backlog/teams-and-agents.md)); `just dev` then seeds a demo team on the Direct engine instead.

## Snapshots: a pending line wrap, and mirrored emulator state

- **State:** terminal snapshots ([Terminal state](./product/architecture/terminal-state.md)) can't restore a pending line wrap: the `charmbracelet/x/vt` emulator keeps it private. `internal/screen/shadow.go` mirrors the pen, scroll region and saved cursor for the same reason. The every-offset round-trip test skips exactly the pending-wrap offsets.
- **Why:** the library exposes no accessors for that state; accepted by the CEO for now.
- **Impact:** a viewer attaching exactly while output sits at the right margin may see one character land a cell off until the program redraws.
- **Fix:** upstream read accessors, then read the state from the emulator and delete `shadow.go`. Tracked in [Snapshots can't restore a pending line wrap](./backlog/snapshot-pending-wrap.md).

## Closing the screen around a data race in `charmbracelet/x/vt`

- **State:** vt v0.1.0's `Emulator.Close` sets a `closed` flag without a lock, and `Emulator.Read` checks it, so closing while `internal/screen` drains the emulator's answers is a data race (found by `just race` on macOS). `screen.VT.Close` works around it: it ends the answers pipe first (through `InputPipe()`, which happens to be an `*io.PipeWriter`), waits for the drain to finish, then closes the emulator. If a vt upgrade changes `InputPipe()`, it falls back to the racy close.
- **Why:** vt is pinned at v0.1.0 behind the `Screen` interface; the race is in its code, not ours.
- **Fix:** report it upstream; drop the workaround once a vt release guards the flag.
