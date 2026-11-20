---
type: note
tags: [architecture]
---

# Technical debt

Workarounds and accepted limitations to revisit. Each item says why it exists and how it would be fixed. When one is scheduled, it becomes a card in the [backlog](./backlog/index.md) that links back here. When it's fixed, remove it from this page.

## Resizing the terminal from the bottom

- **State:** `ui/src/components/terminalFit.ts` writes to xterm.js (never to the agent) around each resize: before a shrink, a scroll of the screen (the cursor along with its line), so the top lines go into the scrollback rather than the rows below the cursor being deleted; before a grow, the cursor on the last row, where xterm.js brings scrollback lines back down, then the cursor back on its line ([Sessions → Body](./product/features/sessions.md)). Output arriving meanwhile is held, so they see the screen they change.
- **Why:** xterm.js's `Buffer.resize` deletes the rows below the cursor first on a shrink, blank or not (its comment assumes they are blank), and brings scrollback back on a grow only with the cursor on the last row. Claude Code keeps its cursor in its input box and, on a resize, repaints the screen from its top as a native terminal would show it: lines vanished from the middle, then (with the shrink alone fixed) doubled at every Line toggle (the CEO, 2026-11-05).
- **Impact:** the scroll ends with an absolute cursor move: with a harness-set scroll region (DECSTBM) or origin mode on, it would land on the wrong line (Claude Code uses neither; Codex sets a region only while inserting history). A cursor in its pending-wrap state (just past the last column) loses that state. With the cursor near the top and text below, only as many lines as sit above it can scroll away; the rest is deleted as before.
- **Fix:** xterm.js resizing anchored at the bottom whatever the cursor's row (worth proposing upstream), then delete `terminalFit.ts`'s writes and keep the plain fit.

## Codex's question tool behind a feature flag

- **State:** Codex agents start with `--enable default_mode_request_user_input` ([Harness adapters → Codex](./product/architecture/harness-adapters.md#codex)), which gives the model its `request_user_input` tool outside plan mode, so an agent can ask the CEO in a dialog as on Claude Code and Pi.
- **Why:** without it, Codex 0.167.0 refuses the tool in its default mode ("unsupported call"), and an agent could only ask in its reply and stop, which raises nothing on the Dashboard. Codex lists the feature as under development.
- **Impact:** a Codex release may rename or drop the feature: an unknown `--enable` could then stop every Codex agent from starting, or the tool could quietly go away again.
- **Fix:** drop the flag once Codex offers the tool by default; until then, check it when the agent image's Codex is updated (`codex features list`).

## Codex's ready opener

- **State:** every Codex session that starts without an opener gets a short one asking it to reply "Ready." ([Harness adapters → Codex](./product/architecture/harness-adapters.md#codex)).
- **Why:** Codex fires no hook before a turn begins (`SessionStart` runs inside the first turn), so a session started with nothing to do never says it is at its prompt: Foreman kept it `running` and its messages never went (the CEO, 2026-11-07).
- **Impact:** one extra model turn per start (a deploy, a wake, a restart), with the resumed conversation as its context; the activity line shows `Working on “Foreman started this session…”` meanwhile. A model that ignores "don't start on anything" may pick its work back up before its messages come.
- **Fix:** drop the opener once Codex reports a session at its prompt without a turn (a `SessionStart` at launch, or another event); check when the agent image's Codex is updated.

## Signing as the CEO from Foreman's process

- **State:** commits in the CEO's name (Merge's squash, the UI's saves of the wiki and the global repo) are signed whenever the CEO's git config says `commit.gpgsign` ([Pipeline and merges → Merge to a target](./product/features/pipeline-and-merges.md#merge-to-a-target)), by git run from Foreman's process; a signature that fails fails the commit, and nothing moves.
- **Why:** everything Foreman commits for the CEO is signed (the CEO, 2026-11-15: "everything that can be signed, is signed"), and their forge rejects unsigned commits at the CEO's own push (the CEO, 2026-11-06).
- **Impact:** Foreman's process must reach the signer: an SSH agent (`SSH_AUTH_SOCK`) or a GPG agent with a cached passphrase, or a pinentry that can ask without a terminal (pinentry-mac does). Started where it can't (a service with no agent), every Merge and every wiki save fails with git's error until the key is reachable; a GPG pinentry that waits on a terminal holds the request until it is cancelled.
- **Fix:** a check when Foreman starts that signing works when the config asks for it, shown on the dashboard's health with git's reason, so a failing setup is found before a merge; when it is first seen.

## Copies of WikiShell's building blocks

- **State:** Foreman renders wikis as [WikiShell](https://github.com/agentic-wiki/wiki-shell) does ([Wiki engine → Rendering](./product/architecture/wiki-engine.md#rendering)) and holds copies from WikiShell v0.13.0: the readable-name rule (`internal/wiki/copied.go`), the board layout (`ordered` and its `knownOrder` table in `internal/wiki/board.go`) and the callout plugin (`ui/src/app/callout.ts`); `ui/src/components/Markdown.tsx` is adapted from its renderer, and `ui/src/components/Frontmatter.tsx` from its `EntryView` (2026-11-02). Heading ids, link destinations, frontmatter references and titles come from the engine (`Heading.ID`, `index.Read`'s links, `Entry.FrontmatterRefs`, `Entry.Title`, wiki v0.17.0).
- **Why:** WikiShell's Go code is under `internal/` and its UI isn't a package, so Foreman can't import either (the CEO, 2026-10-17: copy now, upstream later). WikiShell ships no LICENSE file either.
- **Impact:** a fix in WikiShell doesn't reach Foreman until copied again.
- **Fix:** WikiShell exports its view builders and publishes its renderer as a package; Foreman imports them and deletes the copies. Tracked in [Import WikiShell's building blocks](./backlog/wikishell-upstream.md).

## The wikiview board rename

- **State:** before reading a wiki, Foreman renames its `[[tool.wikiview.board]]` and `[[tool.wikiview.graph]]` tables to the standard's `[[board]]` and `[[graph]]`, and commits `wiki.toml` (`internal/wiki/migrate.go`; [Wiki engine](./product/architecture/wiki-engine.md)). It is line-based: a multi-line string holding a line that looks like an old header would be renamed too.
- **Why:** every Foreman HQ was made with the old key, and reading both keys would mean two paths in every reader (the ADR "Boards and graphs in the standard", 2026-11-19).
- **Impact:** about 100 lines, and a write in every build's path that only finds work once per wiki.
- **Fix:** delete `migrate.go`, its tests and its call in `load`, in Foreman's first release on or after 2027-01-01 (the CEO's call); then a wiki still on the old key shows no boards.

## Old team files' `look`

- **State:** a team file still carrying a member's `[agents.<id>.look]` table (avatars, removed 2026-11-02) loads: `retiredLook` in `internal/home/config.go` keeps it out of the problems, and the next save drops it ([Agent soul](./product/models/agent-soul.md#schema)).
- **Why:** the CEO's existing team files have one; refusing them would block every deployment of those teams until hand-edited.
- **Impact:** one key path is silently ignored rather than reported. A Chief of Staff proposal is still strict (it refuses a `look`).
- **Fix:** remove `retiredLook` (and `decode`'s `retired` parameter, its only user) once no team file in the global repo has a `.look]` table; saving each team once does it.

## Wiki performance: whole-wiki work on every change and read

- **State:** the Wiki block ([Wiki engine](./product/architecture/wiki-engine.md#what-foreman-adds-on-top)) does whole-wiki work in several places:
  - every change rebuilds the wiki's whole index (`index.Build`) and hashes every entry, to find what changed;
  - every image request scans every entry's links to check that one links to it;
  - every entry read reads its file from disk (body, version), and finds its backlinks over the whole index;
  - reading an entry refetches the tree (its "seen" dots), so a `wiki.changed` loads the tree twice.
  - On macOS, fsnotify (kqueue) holds a file handle per watched folder and file, so a very large wiki uses as many handles, and could reach the process's open-file limit.
- **Why:** the simplest correct design, as WikiShell runs it; wikis are expected to hold hundreds of entries, not tens of thousands. Nothing has been measured yet.
- **Impact:** none seen at this size; it grows linearly with the wiki, per change and per image.
- **Fix:** measure on a large wiki first (a generated one of 5k entries). Then, as needed: an incremental refresh contributed upstream to the engine (the spec's rule), a linked-files set kept with each rebuild (WikiShell keeps one) for the image check, and the tree refetched only when the dots change.

## Wiki reading: known edges

- **State:**
  - **Debounce window:** for up to ~200 ms after an edit, an entry's body is read fresh while its links, headings and checkbox lines come from the previous index, so they may disagree until the rebuild (WikiShell has the same window).
  - **A broken wiki reads as none:** a wiki whose `wiki.toml` can't be read (caught mid-write, say) is served as "no wiki" until its next change.
  - **An image in an ignored folder:** a link to an ignored file still resolves, so an entry can show an image kept in a folder `wiki.toml`'s `ignore` leaves out. The watcher doesn't follow such a folder, so the image appearing or going sends no `wiki.changed` until the next change elsewhere in the wiki.
  - **Image symlink race:** the image route resolves symlinks and checks the result is inside the wiki before serving it; an agent with write access could swap a symlink in between. Theoretical: the window is microseconds and the lead's container already holds the wiki.
- **Why:** each is rare or short-lived, and fixing it now costs more than it saves.
- **Fix:** the window: rebuild before reading when a change is pending (Foreman's own writes already rebuild before answering). The symlink race: open the file, then check the opened file's real path (`os.File` plus `/proc` or `F_GETPATH`), if it ever matters.

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


## Foreman's folder: known edges

- **State** (from [Project workspaces](./backlog/archive/project-workspaces.md), 2026-10-28, and [One Foreman folder](./backlog/foreman-folder.md), 2026-11-15):
  - **Worktrees kept on remove:** removing a project without its worktrees leaves them in `worktrees/<project>/`, their records in each code repo, so those branches can't be deleted while checked out there, and a new project of the same id reuses the folders as they are.
  - **Clean up's uncommitted option** keeps the changes git can see (`git.Discard`): files ignored by the repo go with the worktree, and a worktree whose changes are inside a submodule stays, listed as holding changes not committed. The kept changes are pruned with the other backups after 30 days, with nothing in the UI to bring them back: `git show <ref>` or `git checkout <ref> -- <file>` by hand. Card: [Clean up: what its uncommitted option can't keep](./backlog/cleanup-kept-changes.md).
  - **Why:** each is rare, or a deliberate trade next to what the cards had to land.
- **Fix:** a removed project's kept worktrees get an action in Foreman (or are pruned with it); Clean up's gaps in their card.

## Submodules: known edges

- **State** (2026-11-16): Foreman's clones (`repos/`) don't check submodules out, so a private one can't reach the agents from a repo Foreman cloned; renaming a repo fails while an agent's worktree of it has one checked out; a forced worktree removal can lose a commit made only inside its submodule.
- **Why:** the operations added since [Submodules in worktrees](./backlog/submodules-in-worktrees.md) were built without submodules in their tests.
- **Fix:** [Submodules, everywhere Foreman touches a repo](./backlog/submodules-everywhere.md).

## Projects as records: known edges

- **State** (from [Projects as records](./backlog/archive/projects-as-records.md), 2026-10-20):
  - **Locate to a clone:** Locate accepts any folder holding the repo's root commit, so a fresh clone passes; the agents' worktrees still point at the old `.git`, `git worktree repair` can't relink them to another repo, and the next start fails with git's "already exists".
  - **Every start rescans:** each start calls `Checked`, which rescans every project's repos (a few `git` calls each), not only the one starting.
- **Why:** each is rare, cheap or temporary next to what the card had to land.
- **Fix:** Locate refusing a folder whose `.git` isn't the one the worktrees name when worktrees exist (or recreating them); `Checked` rescanning only the project starting.

## One team per project: known edges

- **State** (from [The project as the workplace](./backlog/project-workplace.md), 2026-10-31):
  - **Replacing isn't one transaction:** Deploy stops the team on the project, then writes the new deployment. A failure in between (the database, or a refusal raised only by the store) leaves the project with no team; the old one stays stopped, its worktrees kept, so deploying again recovers.
  - **The Activity filter sees loaded pages only:** filtering by a member or a pair on the map hides what doesn't match among the pages already fetched, so a rare match further back shows only after `Load older`.
  - **The Session section's last agent is kept in memory:** a reload, or another browser, opens the project on the agent named in the URL, else on the lead ([UI state](./product/models/ui-state.md) is planned).
- **Why:** each is rare or cheap next to what the card had to land, and the CEO asked for no workarounds.
- **Fix:** Deploy's stop and write in one store transaction (the processes stopped after it commits); the activity filter sent to `/api/activity` as a query parameter, so the server pages the matches; the last agent saved with the `ui_state` table.

## Reload's busy check is the UI's

- **State:** `Reload` (`ReloadButton`: the project's Team header, the Team page's deployments, the Session) ([Team definition → Reload](./product/models/team-definition.md#reload)) is disabled while an agent is busy, judged in the UI from the statuses it last got; it then calls `POST /api/instances/:id/restart` for each agent behind, which restarts whatever it is told to.
- **Why:** the same shape as Project settings' `Restart them`, and the window is short: statuses arrive as events within a moment.
- **Impact:** an agent that starts a turn between the last status the page got and the click (a message delivered to it) is restarted mid-turn: that turn's unfinished part is lost, its conversation kept. Two tabs clicking at once restart an agent twice.
- **Fix:** a `POST /api/deployments/:id/reload` that checks the statuses on the server and restarts the agents behind in one step, refusing with `409 BUSY`; when a mid-turn restart is seen in use, or with the approval pane's **Reload now?** ([The lead proposes changes to its own team](./backlog/lead-proposes-team.md)).
