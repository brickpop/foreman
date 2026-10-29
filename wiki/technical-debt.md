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

## The workspace migration

- **State:** Foreman moves a project made before workspaces out of `~/.foreman/projects/` and `~/.foreman/sandboxes/` into its workspace (`home.Migrate`, `POST /api/projects/{id}/migrate`, the "Move into a workspace" offer in Project settings), worktrees repaired and the agents' conversations carried over (`harness.Mover` in the Claude Code and Pi adapters); so it still knows the old layout's paths (`projectsDir`, `sandboxesDir`, `legacyHarn`, the legacy branch of `Home.layout` and `Project.HarnessDir`, the watcher's `projects/`).
- **Why:** only the CEO's existing projects need it: Foreman has no other users yet (the CEO, 2026-10-28).
- **Impact:** code, a route and a UI offer kept for a one-off move; the old paths stay named in the code. Its known edges, accepted for a one-off: the moves are renames, so a workspace on another file system than the home is refused by the OS (`EXDEV`, "invalid cross-device link") and nothing moves; a stopped deployment's instance rows keep their old `sandbox_path` until a redeploy writes new ones (nothing reads them meanwhile: a start uses the row a deploy just wrote); a conversation folder Claude Code named with a hash (a path over 200 characters) can't be carried over, so the move is refused; and a failed undo (a rename back that fails too) is ignored, leaving that folder where it got to.
- **Fix:** removed once the CEO has migrated every project: [Remove the workspace migration](./backlog/remove-workspace-migration.md).

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


## Project workspaces: known edges

- **State** (from [Project workspaces](./backlog/project-workspaces.md), 2026-10-28):
  - **Worktrees kept on remove:** removing a project without its worktrees leaves them in `<workspace>/agents/`, their records in each code repo, so those branches can't be deleted while checked out there, and a new project in the same workspace reuses the folders as they are.
  - **A workspace inside an enclosing git repo** (`~/workspace` a repo itself) is refused only when that repo is a code repo of some project; otherwise git run there on the host (the Direct engine) sees the enclosing repo. In a container it isn't mounted.
  - **The workspace is typed, not browsed** in New project and in the move offer: the folder browser lists repos to add, not a place for a new folder.
- **Why:** the card landed the layout; the dialog and the clean-up have cards of their own.
- **Fix:** the folder browser picks the workspace in [Restructure the New project dialog](./backlog/new-project-dialog.md); kept worktrees cleaned up from [Remove a stopped deployment's worktrees](./backlog/remove-worktrees.md).

## Projects as records: known edges

- **State** (from [Projects as records](./backlog/archive/projects-as-records.md), 2026-10-20):
  - **A stopped deployment holds its repos:** the refusal of the same team on two projects sharing a repo counts stopped deployments too, since their worktrees stay. Nothing in the UI removes a stopped deployment's sandboxes yet, so that team can never be deployed on the other project.
  - **Locate to a clone:** Locate accepts any folder holding the repo's root commit, so a fresh clone passes; the agents' worktrees still point at the old `.git`, `git worktree repair` can't relink them to another repo, and the next start fails with git's "already exists".
  - **Every start rescans:** each start calls `Checked`, which rescans every project's repos (a few `git` calls each), not only the one starting.
- **Why:** each is rare, cheap or temporary next to what the card had to land.
- **Fix:** a stopped deployment's sandboxes removed with it (or on demand), and the refusal then counting active ones only; Locate refusing a folder whose `.git` isn't the one the worktrees name when worktrees exist (or recreating them); `Checked` rescanning only the project starting.
