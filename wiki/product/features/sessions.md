---
type: spec
status: planned
tags: [feature, ui, sessions, team-tools]
---

# Sessions

The terminal multiplexer. Every agent [instance](../models/instance.md) is a live harness process with a PTY ([Session runner](../architecture/session-runner.md)); this screen shows its real terminal in the browser, with keyboard input, copy and paste. It is also where the CEO talks to a team: through the lead by default, or to any member directly.

## Top bar

Padding `10px 16px`, bottom border.

- **Left: deployment tabs**, one per active [deployment](../models/deployment.md), labelled `<Team> · <project>` (segmented style), led by a **status dot** for the team: its liveliest agent's status, in the order awaiting the CEO, running, starting, queued, idle, done, asleep, stopped, coloured as the agents' own dots, with the status as its title. A tab whose deployment needs the CEO also shows an amber needs-you dot. **Tabs reorder by drag and drop**: one dropped on another takes its place; the order lasts while the app runs, not across reloads (the CEO, 2026-10-17), and new deployments join at the end. Selecting a tab shows that deployment's agents and loads the lead in a single pane. Each tab ends in a stop icon, a filled square (`--tx4`, `--tx` on hover, title `Stop`; a `×` until 2026-10-21, which read as closing the tab), beside the label rather than inside it: it **stops** that deployment, after the same confirmation as the [Team page](./team-page.md)'s Stop (one dialog, `StopDialog`). Once stopped, its tab is gone.
- **Right**: a **`<project> · Board`** link (outline button) back to the active deployment's project, on its [Board](./board.md) tab: the tasks and their status (the CEO, 2026-10-17); then one segmented control, **`Single | Split | Activity`**, in a 1px `--line` frame ([Design system](../design/design-system.md#buttons)): one terminal, two side by side, or one beside the [Activity](#activity) panel. Split and Activity exclude each other.
- **Empty** (no deployments): centered on `--bg`, `No team is deployed yet, so there are no sessions.` and a primary **`Deploy a team`** button, which opens [Deploy a team](./deploy-team.md) with nothing chosen (the team, the project and the goal).

## The URL

The URL is the view, so a reload, a shared link or Back shows the same thing (found 2026-10-16: a reload could open another agent, picked before the team file had arrived, and nothing was kept): `/sessions?deployment=<team@project>&agent=<agent>&panes=<left>,<right>&view=activity`. `agent` is the active pane's agent; `panes` is there only when split; `view=activity` only while Activity is open. Agents are named by their id (the deployment names the project); a full instance id (`tac@DevProject`) is read too, as notifications link it. Every change replaces the URL (no history entry per click). What the URL names is checked against the deployment's agents: an unknown agent opens the lead, a split that can't be made is one pane, and an agent asked for outside the split takes the left pane. Switching deployment tabs comes back to each one's last view, while the app runs.

## Agent tabs

A strip (`--side`, padding `0 12px`) that scrolls horizontally only.

1. The **lead first**, with the "lead" tag.
2. A 1×20px `--line2` divider and the label **`Direct`** (12px `--tx4`).
3. The other members, in roster order.

Each tab: Avatar 24 with its status dot (`dotBorder --side`), the name (500). Underline (inset 2px bottom): `--acc` if the agent is in the active pane, `--line3` if it is in another pane, none otherwise. Text is `--tx` when shown in a pane, `--tx3` otherwise.

Clicking a tab: if the agent is already in a pane, that pane becomes active; otherwise the agent replaces the active pane's content. Either way the keyboard goes to that pane (the CEO, 2026-10-18): its terminal, or its Line when Line is open, so the CEO types at once; an agent with no session (asleep, queued, stopped) has nothing to take it.

## Panes

One pane, or **two** side by side (equal columns, 1px `--line` divider); two is the maximum in v1. The open panes are remembered per deployment and restored when it is opened again ([UI state](../models/ui-state.md)).

- **Split** adds the first member not in pane 1 as pane 2 and makes it active. **Single** keeps the active pane's agent.
- Clicking (mouse down) in a pane makes it active. When split, the active pane shows a 2px `--acc` top inset.

**Pane header** (padding `9px 16px`, 13px): when split, Avatar 26 with status dot (`dotBorder --term`) and the name (600, opens the [Agent page](./agent-page.md) showing this deployment's prompt); in a single pane both are left out, since the active tab already names the agent. Then `<Harness label> · <project>` (`--tx4`); a spacer; then icon buttons (28px square, radius 6, `--tx3`, `--hover` behind on hover, each labelled for screen readers and in a tooltip): **Copy** (two sheets: copies the whole buffer, scrollback included, as plain text; a tick for 1.4 s), **Line** (a `>_` prompt: toggles the line input below; `--accSoft` / `--accTx` while open), and, always rightmost when shown, **`×`** when split (closes the pane; pane 1 becomes active; the CEO, 2026-10-19).

**Queued agent** (status `queued`: every session slot is taken, idle agents included; the CEO, 2026-10-18): the pane has no terminal, like an asleep one, and says `<Name> is waiting for a session slot: N of M are taken, idle agents included. They start as soon as one frees, and get their messages then.` with a link `Raise the limit in Settings`. It starts by itself when a slot frees ([Put idle agents to sleep](../../backlog/idle-sleep.md) frees them).

**Asleep agent** (status `asleep`: no session runs until it has work, [Agent runtime → Lazy starts](../architecture/agent-runtime.md#lazy-starts)): the pane has no terminal, and its header no `Line` or `Copy`. Centered instead: `<Name> is asleep: no session runs until there's work for them.` (13.5px `--tx3`) and an outline **`Start <Name>`** button, which wakes it; the terminal appears once it runs. A failure shows an error toast. A **stopped** agent (two crashes, or a start that failed) gets the same pane: `<Name> stopped: <reason>. Messages for them wait until you start them again; they pick up their conversation.` and **`Start <Name> again`**. The reason is the instance's `stop_reason` ([Instance](../models/instance.md)): `couldn't start: <why>` (a repo without its main branch, say) or `it exited twice in a row (code <n>)`; without one (a stop from before reasons were kept), `<Name> stopped.` (the CEO, 2026-10-21). An agent that exits cleanly (`/exit`) is asleep ([Agent runtime → Restarts](../architecture/agent-runtime.md#restarts)).

**Direct-line banner** on every non-lead pane (padding `7px 16px`, 12.5px `--tx3`, `--side`, bottom border): `Direct line to <Name>. They'll tell <Lead>, the lead, if it matters.`

**Body**: a real terminal emulator on `--term`, in the CEO's terminal font (JetBrains Mono 13px by default, [Settings](./settings.md#appearance)), line height 1.25.
- Full ANSI (colors, cursor movement, alternate screen), as the harness renders it.
- Scrollback is replayed from the session log when a pane opens ([Persistence](../architecture/persistence.md)); the live stream follows.
- Native text selection, copy and paste; clickable links. When the harness turns on mouse reporting (Claude Code does in some views), a drag goes to it rather than selecting: Option-drag on a Mac (Shift-drag elsewhere) still selects (found by the CEO, 2026-10-16).
- **Shift+Enter** types the harness's newline keystroke (a new line without sending), the same as in the line input: a terminal alone can't tell it from Enter.
- The pane size is propagated to the PTY on every resize. One PTY has one size, so when several panes or browser tabs show the same instance, **the most recently focused view sets it**; the others render that size (letterboxed or scrolled).
- Opening a pane (or reconnecting) replays the screen and its scrollback; it is drawn hidden and shown at the bottom once drawn (a 150 ms fade), so it never flashes by scrolling (at most 600 ms hidden if nothing arrives).
- Auto-scroll follows output while the view is within 160px of the bottom; scrolling up stops following.
- Keystrokes go straight to the PTY, raw, always ([API](../architecture/api.md), `WS /ws/pty/:instance`). Foreman doesn't interpret them.

**Messages wait on the CEO's draft** (the CEO's request, 2026-10-16). Text the CEO types straight into an agent's terminal and hasn't sent yet is a **draft**, and a delivery would land on it. So while one is there, that agent's messages wait, and an amber box above the line input says so: inset from the pane's edges (`8px 12px`), radius 8, a 2px `--acc` border on all four sides, `--accSoft`, `--accTx`, so it is never mistaken for the harness's own status line nor half hidden by the Activity panel. It says `1 message waiting for your line: send or clear it. 0:42`, with a tiny **`Pause`** / **`Resume`** button. The countdown starts at 1:00 and runs only while the CEO isn't typing (5 s after the last key), so they aren't rushed; `(waits while you type)` says so, and Pause holds it until resumed. Sending the line lets the messages follow at the agent's next idle (or at once, if the Enter started nothing); clearing it (Ctrl+U, Ctrl+C, Backspace to empty) lets them through at once. At 0:00 they go after the draft, separated by a blank line, `---` and `The CEO's text above is an unfinished draft: don't act on it yet.` ([Team tools → Delivery](../architecture/team-tools.md#delivery)). Foreman tells a draft from other keys by what is typed: printable text makes it, Enter, Ctrl+C and Ctrl+U end it, arrows and other escape sequences change nothing. Accepted: text typed into a harness's own dialog counts as a draft too, and clearing the prompt with the harness's own keys (Claude Code's Esc Esc) isn't seen, so the messages wait for the countdown.

**Line input** (optional mode, for paste or mobile): a light box under the terminal (margin `4px 12px 12px`, radius 8, 1px `--line2` turning `--line3` on focus instead of the focus ring, `--term`, padding `12px 16px`) holding an amber `❯` (Mono 500 13px `--accTx`) and a borderless text box (line-height 1.6) that grows with its lines (up to 8). Enter sends it through Foreman (`POST /api/instances/:id/line`), which types it the way it types a delivery and in the same queue, so the two never mix: each line, each line break as the harness's **newline keystroke** ([Harness adapters](../architecture/harness-adapters.md)) after a short pause, then Enter after a pause. Sent in one burst, the Enter reached Claude Code as part of a paste (a line break, not sending; found 2026-10-16). A refusal (the agent stopped) shows beside the box. Shift+Enter starts a new line; an empty input still sends Enter. Typing `@` (at the start or after a space) offers the teammates' names, starting with what follows it (a small list above the box; the arrows choose, Enter or Tab takes one, Escape closes the list, a click works too); the name goes in plain, without the `@`, since a harness may read `@` as its own file mention (Claude Code does, and its file list would take the Enter). The mode is switched on by a **`Line`** button in the pane header, or **Ctrl+Shift+L** (**Cmd+Shift+L** on a Mac), anywhere on the screen, for the active pane (caught before the terminal, so the agent never gets it; Ctrl+L alone, and on a Mac every Ctrl key, still reach the agent). Opening Line puts the keyboard in it. The shortcut from the terminal goes back into an open Line; from Line, it closes it. Escape or the button also close it. Closing puts the keyboard back in the terminal. Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [Team tools](../architecture/team-tools.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. It goes raw to the member's terminal under the direct-line banner; Foreman posts nothing to the lead. The member tells the lead itself when the instruction matters to the team, as its [session prompt](../architecture/session-prompt.md#talking-to-the-ceo) asks.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

## Activity

The terminal shows only what the agent's program prints. Anything Foreman wrote into it would be typed input to the agent ([Protocol → Deliveries and markers](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)), so the team's activity (its messages and git actions) is shown in one optional panel beside the panes, never in them (the CEO's decision, 2026-10-16: it replaces the thin marker strip along each pane and the Messages drawer, which said little). The **`Activity`** view in the top bar opens it. Split and Activity exclude each other (two terminals, or one beside the Activity panel): opening one closes the other.

A right-hand column (**360px**, `--panel`, left border) for the selected deployment only ([Team message](../models/team-message.md)). It narrows the panes rather than covering them.

- **Header:** "Activity" (600), subline `<Team> on <project> only` (12.5px `--tx4`), `×`.
- **Names are labels** everywhere in the panel, in the primary colour: a small box (`--accSoft`, 1px `--accLine`, radius 5, 12px 600, `--accTx`); the CEO's (`You`, `--raised`, `--tx`) and Foreman's (muted) stand apart.
- **An exchange reads `<agent> <verb> <agent>`:** the verb, in the past tense and its kind's colour, needs no box and no arrow (it says the direction): `asked` (`--info`), `replied to` (`--ok`), `handed off to` (`--tx2`), `wrote to` and `notified` (`--tx3`). Names keep their labels.
- **Who calls whom** (compact, 150px tall): a name box per agent on an ellipse (in the primary colour, filled `--acc` when it is the filter), with its status dot (and the word `queued` when it waits for a session slot: the dashed dot is too small to tell from idle; the box's tooltip names the status), plus the CEO when they sent something. Foreman is never on the map (the CEO, 2026-10-17): its notices (a merge done, a conflict, `main` moving) are system news, not teamwork, so they show in the list only. A line joins each pair that exchanged messages, thicker with more of them, `--line3`, or `--acc` while a message between them waits; at its middle a small count badge (Mono 10.5px) whose tooltip gives both directions. Clicking a name box shows only its messages; clicking a badge, only that pair's; again (or `Show all` in the strip that names the filter) shows everything.
- **The exchanges, newest activity first, one row per thread** (an ask and its replies; a hand-off, a contribution or a notice on its own): the exchange (`Tic asked Tac`), a spacer, how it stands (`no reply yet`, `1 reply`, `waiting` in `--accTx`, `expired`, `failed`), the time of its last message (local: `14:05` today, `Oct 15 14:05` before); under it the first message's start, on one line. A click opens the thread as a chat: each message a bubble (the asker's on the left, `--raised`; the other side's on the right, `--panel`), with its sender's label, `kind · msg-N`, the time, the whole text (selectable; its `inline code` and ``` fenced blocks ``` shown as code, the rest as plain text, never HTML), the artifact, `Waiting: <why> (the recipient's state: not started because every session slot is taken, idle agents included, with the count; busy until its turn ends; waiting for the CEO; asleep and being woken; the CEO's unsent line in its terminal), the row's `waiting` carrying the same as its tooltip` or a failure's reason; then `Open <asker>'s pane`.

- **Git actions** (`switch`, `commit`, `merge`, `abort_merge`, `set_submodule` (its commit message, which says what it did), and the lead's `request_merge`, which names each merge request it made and links **Review it**, which opens the project's Overview; [Protocol → Team actions](../architecture/protocol.md#team-actions)) sit between the threads, one row each, newest first, on a faint `--side` tint: the agent's label and what it did (`committed "cal v0"` for one repo, `committed api, project` for several, each repo's line then ending with its own message; `switched to wt/tac/LED-44 (new)`, `merged wt/tac/main`, `aborted a merge`; `tried to …` and `refused` in `--err` when refused), the time; under it each repo (Mono 11.5px: the repo, its branch, the commit's 7 characters, conflicts in `--accTx`), or the refusal's text. Filtering by an agent shows its actions; filtering by a pair leaves them out.

What an agent itself receives (a teammate's ask, a reply, the CEO's answer) is a **delivery**: typed into its session when it is idle, so it appears in the terminal as the harness shows any input. The texts are in [Protocol → Team tools](../architecture/protocol.md#team-tools).

**Later:** Foreman's other notes for the CEO (`instance.marker` events: escalations, answers, expiries, system notes, with the glyphs in [Team tools → Markers](../architecture/team-tools.md#markers-notes-for-the-ceo-never-in-the-terminal)) join the list as rows of their own.
