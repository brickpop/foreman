---
type: task
status: todo
priority: low
tags: [bug]
epic: terminal-multiplexer
---

# Snapshots can't restore a pending line wrap

**The glitch:** a [snapshot](../product/architecture/terminal-state.md) can't restore a pending line wrap, because the `charmbracelet/x/vt` emulator keeps that state private. A viewer that attaches exactly while output sits at the right margin may see the next character land one cell off, until the program redraws. Related: `internal/screen/shadow.go` mirrors the pen, the scroll region and the saved cursor, because the emulator doesn't expose them either.

**Fix:** propose read accessors upstream to `charmbracelet/x/vt` (pending wrap, cursor pen, scroll region, saved cursor). Once released, use them and delete `shadow.go`. The every-offset round-trip test then stops skipping: today it skips exactly the pending-wrap offsets (18 of 639), found with an exact probe.

- [ ] open the upstream issue or PR with the accessors
- [ ] when released: bump the pinned version, read the state from the emulator, delete `shadow.go`
- [ ] remove the skip in `TestSnapshotAtEveryByteOffset`; remove the [technical debt](../technical-debt.md) entry
