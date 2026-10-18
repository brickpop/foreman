---
type: note
tags: [architecture]
---

# Technical debt

Workarounds and accepted limitations to revisit. Each item says why it exists and how it would be fixed. When one is scheduled, it becomes a card in the [backlog](./backlog/index.md) that links back here. When it's fixed, remove it from this page.

## Copies of wikiview's building blocks

- **State:** Foreman renders wikis as [wikiview](https://github.com/agentic-wiki/wikiview) does ([Wiki engine → Rendering](./product/architecture/wiki-engine.md#rendering)) and holds copies from wikiview v0.13.0: the heading-id rule and the readable-name rule (`internal/wiki/copied.go`), the board layout (`ordered` and its `knownOrder` table in `internal/wiki/board.go`) and the callout plugin (`ui/src/app/callout.ts`); `ui/src/components/Markdown.tsx` is adapted from its renderer. The heading-id rule is the engine's, which keeps it unexported, so the copy breaks the wiki engine's "never reimplement an engine rule"; a test cross-checks it against `wiki check`.
- **Why:** wikiview's Go code is under `internal/` and its UI isn't a package, so Foreman can't import either (the CEO, 2026-10-17: copy now, upstream later). wikiview ships no LICENSE file either.
- **Impact:** a fix in wikiview or a change to the engine's slug rule doesn't reach Foreman until copied again (the cross-check catches the slug rule).
- **Fix:** the engine exports its heading-id rule; wikiview exports its view builders and publishes its renderer as a package; Foreman imports them and deletes the copies. Tracked in [Import wikiview's building blocks](./backlog/wikiview-upstream.md).

## Wiki performance: whole-wiki work on every change and read

- **State:** the Wiki block ([Wiki engine](./product/architecture/wiki-engine.md#what-foreman-adds-on-top)) does whole-wiki work in several places:
  - every change rebuilds the wiki's whole index (`index.Build`) and hashes every entry, to find what changed;
  - every image request scans every entry's links to check that one links to it;
  - every entry read reads its file from disk (body, version), and finds its backlinks over the whole index;
  - reading an entry refetches the tree (its "seen" dots), so a `wiki.changed` loads the tree twice.
  - On macOS, fsnotify (kqueue) holds a file handle per watched folder and file, so a very large wiki uses as many handles, and could reach the process's open-file limit.
- **Why:** the simplest correct design, as wikiview runs it; wikis are expected to hold hundreds of entries, not tens of thousands. Nothing has been measured yet.
- **Impact:** none seen at this size; it grows linearly with the wiki, per change and per image.
- **Fix:** measure on a large wiki first (a generated one of 5k entries). Then, as needed: an incremental refresh contributed upstream to the engine (the spec's rule), a linked-files set kept with each rebuild (wikiview keeps one) for the image check, and the tree refetched only when the dots change.

## Wiki reading: known edges

- **State:**
  - **Debounce window:** for up to ~200 ms after an edit, an entry's body is read fresh while its links, headings and checkbox lines come from the previous index, so they may disagree until the rebuild (wikiview has the same window).
  - **A broken wiki reads as none:** a wiki whose `wiki.toml` can't be read (caught mid-write, say) is served as "no wiki" until its next change.
  - **Re-encoded links:** a link whose href react-markdown re-encodes (spaces, some Unicode) misses its lookup in the entry's link table and is treated as an external link.
  - **Image symlink race:** the image route resolves symlinks and checks the result is inside the wiki before serving it; an agent with write access could swap a symlink in between. Theoretical: the window is microseconds and the lead's container already holds the wiki.
- **Why:** each is rare or short-lived, and fixing it now costs more than it saves.
- **Fix:** the window: rebuild before reading when a change is pending (Foreman's own writes already rebuild before answering). Re-encoded links: look links up by their decoded form too (upstream in wikiview first, since its renderer has the same gap). The symlink race: open the file, then check the opened file's real path (`os.File` plus `/proc` or `F_GETPATH`), if it ever matters.

## Snapshots: a pending line wrap, and mirrored emulator state

- **State:** terminal snapshots ([Terminal state](./product/architecture/terminal-state.md)) can't restore a pending line wrap: the `charmbracelet/x/vt` emulator keeps it private. `internal/screen/shadow.go` mirrors the pen, scroll region and saved cursor for the same reason. The every-offset round-trip test skips exactly the pending-wrap offsets.
- **Why:** the library exposes no accessors for that state; accepted by the CEO for now.
- **Impact:** a viewer attaching exactly while output sits at the right margin may see one character land a cell off until the program redraws.
- **Fix:** upstream read accessors, then read the state from the emulator and delete `shadow.go`. Tracked in [Snapshots can't restore a pending line wrap](./backlog/snapshot-pending-wrap.md).

## Closing the screen around a data race in `charmbracelet/x/vt`

- **State:** vt v0.1.0's `Emulator.Close` sets a `closed` flag without a lock, and `Emulator.Read` checks it, so closing while `internal/screen` drains the emulator's answers is a data race (found by `just race` on macOS). `screen.VT.Close` works around it: it ends the answers pipe first (through `InputPipe()`, which happens to be an `*io.PipeWriter`), waits for the drain to finish, then closes the emulator. If a vt upgrade changes `InputPipe()`, it falls back to the racy close.
- **Why:** vt is pinned at v0.1.0 behind the `Screen` interface; the race is in its code, not ours.
- **Fix:** report it upstream; drop the workaround once a vt release guards the flag.

## Delivery types into whatever Claude Code shows

- **State:** Messaging types a line when the hooks say the agent is idle (`SessionStart`, `Stop`). A startup dialog Foreman doesn't know about also comes after `SessionStart`, so the line answers the dialog, and the message is marked `delivered` but lost. Found live: Claude Code's "Try the new fullscreen renderer?" took Mara's first ask to Tove (fixed by pinning `tui`).
- **Why:** a harness gives no acknowledgement that a line reached its prompt; each known dialog is switched off in the config Foreman writes.
- **Fix:** the Claude Code integration tests (`just test-claude`) start a fresh session on each Claude Code update and fail on a new dialog. If dialogs keep coming, check the screen for the prompt before typing.

