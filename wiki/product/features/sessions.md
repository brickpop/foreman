---
type: spec
status: built
tags: [feature, ui, sessions, team-tools]
---

# Sessions

The terminal multiplexer, as a project's **Session** section ([Project page](./project-page.md)). Every agent [instance](../models/instance.md) is a live harness process with a PTY ([Session runner](../architecture/session-runner.md)); the section shows its real terminal in the browser, with keyboard input, copy and paste. It is also where the CEO talks to the team on the project: through the lead by default, or to any member directly.

A project has one team at most ([Deployment](../models/deployment.md#invariants)), so its Session is that team's. There is no list of sessions any more: the CEO follows projects, and each project's Session is in the sidebar under it (the CEO, 2026-10-31: the Sessions screen and its list, `/sessions`, went with the [Project page](./project-page.md); two sessions side by side are still two browser tabs).

## The section

**Header** (padding `10px 16px`, bottom border): the team's **status dot** (its liveliest agent's status, in the order awaiting the CEO, running, starting, queued, idle, done, asleep, stopped, coloured as the agents' own dots, with the status as its title), the team's name (600 15px, a link to its [Team page](./team-page.md)) and `· <project>` (`--tx3`), the shown agent's branch ([Agent tabs](#agent-tabs)), and an attention needs-you dot when a request of the team is open. Nothing else: the project's Inbox, Board, Deployment (Stop included) and the rest are in the sidebar.

**No team on the project**: an empty state, `No team is on <project>, so there are no sessions. Deploy one.`, with a primary **`Deploy a team`** opening [Deploy a team](./deploy-team.md) on the project, the team that worked here last chosen at first.

## The URL

The URL is the view, so a reload, a shared link or Back shows the same agent (found 2026-10-16: a reload could open another agent, picked before the team file had arrived, and nothing was kept): `/projects/<project>/session?agent=<agent>`. The agent is named by its id (the project is in the path); a full instance id (`tac@ledger`) is read too, as notifications link it. With no agent, or one not on the team (the one replaced, one gone since a team change), it is the last agent shown on the project while the app runs, else the lead. The section then rewrites the URL to name the agent it shows (replacing it, no history entry); every tab click replaces it too. Deploy opens the new team's Session only once its agents are loaded (it waits for them), or the section would take the project for one with no team.

## Agent tabs

A strip (`--side`, padding `0 12px`) whose tabs scroll horizontally only.

1. The **lead first**, with the "lead" tag.
2. A 1×20px `--line2` divider and the label **`Direct`** (12px `--tx4`).
3. The other members, in roster order.

Each tab: Avatar 24 with its status dot (`dotBorder --side`), the name (500). The agent shown is underlined (inset 2px bottom, `--acc`) and `--tx`, the others `--tx3`. **Its branch shows in the header**, right after `<team> · <project>` (Mono 11.5px `--tx4`; the CEO, 2026-10-25, moved out of the tab 2026-10-27): one per agent, since `switch` moves all its code repos together; should its repos ever differ, each branch, comma-separated. It comes from the project's repos (`GET /api/projects/:id/repos`, each branch with the agent that has it checked out), refreshed by every git action (`team.action`); none shows before the agent's first start makes its worktrees.

At the strip's right end, always visible: the agent's **role** and harness (`<Role> · <Harness label>`, 12.5px `--tx4`; the CEO, 2026-10-25: the role says more than the old direct-line banner), then icon buttons (28px square, radius 6, `--tx3`, `--hover` behind on hover, each labelled for screen readers and in a tooltip): **Copy** (two sheets: copies the whole buffer, scrollback included, as plain text; a tick for 1.4 s) and **Line** (a `>_` prompt: toggles the line input; `--accSoft` / `--accTx` while open). An agent with no session has neither.

Clicking a tab shows that agent, and the keyboard goes to it (the CEO, 2026-10-18): its terminal, or its Line when Line is open, so the CEO types at once; an agent with no session (asleep, queued, stopped) has nothing to take it. **`⌘⇧←` / `⌘⇧→`** (`Ctrl+Shift+←` / `→` off a Mac) do the same for the previous / next tab, round the ends, from anywhere on the page (the CEO, 2026-10-30: without the mouse); like Line's shortcut, the window hears them before the terminal, so the agent never gets them, and in the Line input they no longer select to the line's start / end (off a Mac, by word).

## The terminal

The agent shown fills the section under its tabs. The team's messages and git actions are in the project's [Activity](./project-activity.md) section, never in the terminal.

**Queued agent** (status `queued`: every session slot is taken, idle agents included; the CEO, 2026-10-18): no terminal shows, as for an asleep one; the page says `<Name> is waiting for a session slot: N of M are taken, idle agents included. They start as soon as one frees, and get their messages then.` with a link `Raise the limit in Settings`. It starts by itself when a slot frees (an idle agent going to sleep frees one: [Agent runtime → Idle agents make room](../architecture/agent-runtime.md#idle-agents-make-room)).

**Asleep agent** (status `asleep`: no session runs until it has work, [Agent runtime → Lazy starts](../architecture/agent-runtime.md#lazy-starts)): no terminal, and no `Line` or `Copy`. Centered instead: `<Name> is asleep: no session runs until there's work for them.` (13.5px `--tx3`) and an outline **`Start <Name>`** button, which wakes it; the terminal appears once it runs. A failure shows an error toast. A **stopped** agent (two crashes, or a start that failed) gets the same: `<Name> stopped: <reason>. Messages for them wait until you start them again; they pick up their conversation.` and **`Start <Name> again`**. The reason is the instance's `stop_reason` ([Instance](../models/instance.md)): `couldn't start: <why>` (a repo without its main branch, say) or `it exited twice in a row (code <n>)`; without one (a stop from before reasons were kept), `<Name> stopped.` (the CEO, 2026-10-21). An agent that exits cleanly (`/exit`) is asleep ([Agent runtime → Restarts](../architecture/agent-runtime.md#restarts)).

**Body**: a real terminal emulator on `--term`, in the CEO's terminal font (JetBrains Mono 13px by default, [Settings](./settings.md#appearance)), line height 1.25.
- Full ANSI (colors, cursor movement, alternate screen), as the harness renders it.
- Scrollback is replayed from the session log when a terminal opens ([Persistence](../architecture/persistence.md)); the live stream follows.
- Native text selection, copy and paste; clickable links. When the harness turns on mouse reporting (Claude Code does in some views), a drag goes to it rather than selecting: Option-drag on a Mac (Shift-drag elsewhere) still selects (found by the CEO, 2026-10-16).
- **Shift+Enter** types the harness's newline keystroke (a new line without sending), the same as in the line input: a terminal alone can't tell it from Enter.
- The terminal size is propagated to the PTY on every resize. One PTY has one size, so when several browser tabs show the same instance, **the most recently focused view sets it**; the others render that size (letterboxed or scrolled).
- Opening a terminal (or reconnecting) replays the screen and its scrollback; it is drawn hidden and shown at the bottom once drawn (a 150 ms fade), so it never flashes by scrolling (at most 600 ms hidden if nothing arrives).
- Auto-scroll follows output while the view is within 160px of the bottom; scrolling up stops following.
- Keystrokes go straight to the PTY, raw, always ([API](../architecture/api.md), `WS /ws/pty/:instance`). Foreman doesn't interpret them.

**Messages wait on the CEO's draft** (the CEO's request, 2026-10-16). Text the CEO types straight into an agent's terminal and hasn't sent yet is a **draft**, and a delivery would land on it. So while one is there, that agent's messages wait, and an attention box above the line input says so: inset from the edges (`8px 12px`), radius 8, a 2px `--attn` border on all four sides, `--attnSoft`, `--attnTx`, so it is never mistaken for the harness's own status line. It says `1 message waiting for your line: send or clear it. 0:42`, with a tiny **`Pause`** / **`Resume`** button. The countdown starts at 1:00 and runs only while the CEO isn't typing (5 s after the last key), so they aren't rushed; `(waits while you type)` says so, and Pause holds it until resumed. Sending the line (from the terminal or the Line input) lets the messages follow at the agent's next idle (or at once, if the Enter started nothing), and until they are in, the box stays, without countdown or Pause: `1 message waiting: it goes in when Mara's turn ends.` (the CEO saw the box vanish on sending and took the messages for lost, 2026-10-30); clearing it (Ctrl+U, Ctrl+C, Backspace to empty) lets them through at once. At 0:00 they go after the draft, separated by a blank line, `---` and `The CEO's text above is an unfinished draft: don't act on it yet.` ([Team tools → Delivery](../architecture/team-tools.md#delivery)). Foreman tells a draft from other keys by what is typed: printable text makes it, Enter, Ctrl+C and Ctrl+U end it, arrows and other escape sequences change nothing. Accepted: text typed into a harness's own dialog counts as a draft too, and clearing the prompt with the harness's own keys (Claude Code's Esc Esc) isn't seen, so the messages wait for the countdown.

**Line input** (optional mode, for paste or mobile): a light box under the terminal (margin `4px 12px 12px`, radius 8, 1px `--line2` turning `--line3` on focus instead of the focus ring, `--term`, padding `12px 16px`) holding an accent `❯` (Mono 500 13px `--accTx`) and a borderless text box (line-height 1.6) that grows with its lines (up to 8). Enter sends it through Foreman (`POST /api/instances/:id/line`), which types it the way it types a delivery and in the same queue, so the two never mix: each line, each line break as the harness's **newline keystroke** ([Harness adapters](../architecture/harness-adapters.md)) after a short pause, then Enter after a pause. Sent in one burst, the Enter reached Claude Code as part of a paste (a line break, not sending; found 2026-10-16). A refusal (the agent stopped) shows beside the box. Shift+Enter starts a new line; an empty input still sends Enter. Typing `@` (at the start or after a space) offers the teammates' names, starting with what follows it (a small list above the box; the arrows choose, Enter or Tab takes one, Escape closes the list, a click works too); the name goes in plain, without the `@`, since a harness may read `@` as its own file mention (Claude Code does, and its file list would take the Enter). The mode is switched on by the **`Line`** button at the end of the agent tabs, or **Ctrl+Shift+L** (**Cmd+Shift+L** on a Mac), anywhere on the screen (caught before the terminal, so the agent never gets it; Ctrl+L alone, and on a Mac every Ctrl key, still reach the agent). Opening Line puts the keyboard in it. The shortcut from the terminal goes back into an open Line; from Line, it closes it. Escape or the button also close it. Closing puts the keyboard back in the terminal. Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [Team tools](../architecture/team-tools.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. It goes raw to the member's terminal; Foreman posts nothing to the lead. The member tells the lead itself when the instruction matters to the team, as its [session prompt](../architecture/session-prompt.md#talking-to-the-ceo) asks.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

