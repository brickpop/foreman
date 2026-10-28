---
type: spec
status: planned
tags: [feature, ui, sessions, team-tools]
---

# Sessions

The terminal multiplexer. Every agent [instance](../models/instance.md) is a live harness process with a PTY ([Session runner](../architecture/session-runner.md)); this screen shows its real terminal in the browser, with keyboard input, copy and paste. It is also where the CEO talks to a team: through the lead by default, or to any member directly.

## The list

`/sessions`, the sidebar's **Sessions** (the CEO, 2026-10-28: the deployments aren't listed in the sidebar, which stays short). The title `Sessions`, under it the Dashboard's line (`<n> sessions working across <n> deployments.`), and on the right a primary **`Deploy a team`**, which opens [Deploy a team](./deploy-team.md) with nothing chosen. Then one section per project with an active deployment, by project name, labelled with the project; in it one card per deployment, by team: the [Dashboard's At work card](./dashboard.md#at-work), the same component (`DeploymentCard`), with its team, needs-you dot, goal, awake agents and what each is doing, and its asleep agents counted. The card opens the deployment's page, coming back to the agent last shown there; an agent's row opens that agent. Only active deployments: a stopped one is reached from its [Team page](./team-page.md), where it is redeployed.

**Empty** (no deployments): under the title, a dashed box, `No team is deployed yet, so there are no sessions.`

## A page per deployment

Each active [deployment](../models/deployment.md) is a page of its own (the CEO, 2026-10-25: two deployments, or two agents, side by side are two browser tabs; the deployment tabs, their drag-to-reorder and Split are gone), opened from [the list](#the-list) or from any link to a session elsewhere (Dashboard, notifications, Project, Pipeline, Deploy).

**Header** (padding `10px 16px`, bottom border): the deployment's **status dot** (its liveliest agent's status, in the order awaiting the CEO, running, starting, queued, idle, done, asleep, stopped, coloured as the agents' own dots, with the status as its title), `<Team>` (600 15px) and `· <project>` (`--tx3`), an amber needs-you dot when it needs the CEO; a spacer; then **`Board`** (outline link: the project's [Board](./board.md) tab, its tasks and their status; the CEO, 2026-10-17), **`Activity`** (an outline toggle, `--raised` and `--line3` while on: the [Activity](#activity) panel beside the terminal), and a danger **`Stop`**, which stops the deployment after the same confirmation as the [Team page](./team-page.md)'s Stop (one dialog, `StopDialog`). Once stopped, the page shows [the list](#the-list).

## The URL

The URL is the view, so a reload, a shared link or Back shows the same thing (found 2026-10-16: a reload could open another agent, picked before the team file had arrived, and nothing was kept): `/sessions?deployment=<team@project>&agent=<agent>&view=activity`. `view=activity` is there only while Activity is open. The agent is named by its id (the deployment names the project); a full instance id (`tac@DevProject`) is read too, as notifications link it. What the URL names is checked against what is deployed: no deployment, or one not deployed (stopped since), is [the list](#the-list) (`/sessions`); an unknown agent is the lead. The page then rewrites the URL to name exactly what it shows (replacing it, no history entry); every click replaces it too. A URL naming only the deployment (the list's link) comes back to its last agent and Activity, while the app runs. A new deployment's page opens only once its agents are loaded (Deploy waits for them), or it would be taken for one not deployed.

## Agent tabs

A strip (`--side`, padding `0 12px`) whose tabs scroll horizontally only.

1. The **lead first**, with the "lead" tag.
2. A 1×20px `--line2` divider and the label **`Direct`** (12px `--tx4`).
3. The other members, in roster order.

Each tab: Avatar 24 with its status dot (`dotBorder --side`), the name (500). The agent shown is underlined (inset 2px bottom, `--acc`) and `--tx`, the others `--tx3`. **Its branch shows in the header**, right after `<team> · <project>` (Mono 11.5px `--tx4`; the CEO, 2026-10-25, moved out of the tab 2026-10-27): one per agent, since `switch` moves all its code repos together; should its repos ever differ, each branch, comma-separated. It comes from the project's repos (`GET /api/projects/:id/repos`, each branch with the agent that has it checked out), refreshed by every git action (`team.action`); none shows before the agent's first start makes its worktrees.

At the strip's right end, always visible: the agent's **role** and harness (`<Role> · <Harness label>`, 12.5px `--tx4`; the CEO, 2026-10-25: the role says more than the old direct-line banner), then icon buttons (28px square, radius 6, `--tx3`, `--hover` behind on hover, each labelled for screen readers and in a tooltip): **Copy** (two sheets: copies the whole buffer, scrollback included, as plain text; a tick for 1.4 s) and **Line** (a `>_` prompt: toggles the line input; `--accSoft` / `--accTx` while open). An agent with no session has neither.

Clicking a tab shows that agent, and the keyboard goes to it (the CEO, 2026-10-18): its terminal, or its Line when Line is open, so the CEO types at once; an agent with no session (asleep, queued, stopped) has nothing to take it.

## The terminal

The agent shown fills the page under its tabs, with the Activity panel beside it when open.

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

**Messages wait on the CEO's draft** (the CEO's request, 2026-10-16). Text the CEO types straight into an agent's terminal and hasn't sent yet is a **draft**, and a delivery would land on it. So while one is there, that agent's messages wait, and an amber box above the line input says so: inset from the edges (`8px 12px`), radius 8, a 2px `--acc` border on all four sides, `--accSoft`, `--accTx`, so it is never mistaken for the harness's own status line nor half hidden by the Activity panel. It says `1 message waiting for your line: send or clear it. 0:42`, with a tiny **`Pause`** / **`Resume`** button. The countdown starts at 1:00 and runs only while the CEO isn't typing (5 s after the last key), so they aren't rushed; `(waits while you type)` says so, and Pause holds it until resumed. Sending the line lets the messages follow at the agent's next idle (or at once, if the Enter started nothing); clearing it (Ctrl+U, Ctrl+C, Backspace to empty) lets them through at once. At 0:00 they go after the draft, separated by a blank line, `---` and `The CEO's text above is an unfinished draft: don't act on it yet.` ([Team tools → Delivery](../architecture/team-tools.md#delivery)). Foreman tells a draft from other keys by what is typed: printable text makes it, Enter, Ctrl+C and Ctrl+U end it, arrows and other escape sequences change nothing. Accepted: text typed into a harness's own dialog counts as a draft too, and clearing the prompt with the harness's own keys (Claude Code's Esc Esc) isn't seen, so the messages wait for the countdown.

**Line input** (optional mode, for paste or mobile): a light box under the terminal (margin `4px 12px 12px`, radius 8, 1px `--line2` turning `--line3` on focus instead of the focus ring, `--term`, padding `12px 16px`) holding an amber `❯` (Mono 500 13px `--accTx`) and a borderless text box (line-height 1.6) that grows with its lines (up to 8). Enter sends it through Foreman (`POST /api/instances/:id/line`), which types it the way it types a delivery and in the same queue, so the two never mix: each line, each line break as the harness's **newline keystroke** ([Harness adapters](../architecture/harness-adapters.md)) after a short pause, then Enter after a pause. Sent in one burst, the Enter reached Claude Code as part of a paste (a line break, not sending; found 2026-10-16). A refusal (the agent stopped) shows beside the box. Shift+Enter starts a new line; an empty input still sends Enter. Typing `@` (at the start or after a space) offers the teammates' names, starting with what follows it (a small list above the box; the arrows choose, Enter or Tab takes one, Escape closes the list, a click works too); the name goes in plain, without the `@`, since a harness may read `@` as its own file mention (Claude Code does, and its file list would take the Enter). The mode is switched on by the **`Line`** button at the end of the agent tabs, or **Ctrl+Shift+L** (**Cmd+Shift+L** on a Mac), anywhere on the screen (caught before the terminal, so the agent never gets it; Ctrl+L alone, and on a Mac every Ctrl key, still reach the agent). Opening Line puts the keyboard in it. The shortcut from the terminal goes back into an open Line; from Line, it closes it. Escape or the button also close it. Closing puts the keyboard back in the terminal. Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [Team tools](../architecture/team-tools.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. It goes raw to the member's terminal; Foreman posts nothing to the lead. The member tells the lead itself when the instruction matters to the team, as its [session prompt](../architecture/session-prompt.md#talking-to-the-ceo) asks.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

## Activity

The terminal shows only what the agent's program prints. Anything Foreman wrote into it would be typed input to the agent ([Protocol → Deliveries and markers](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)), so the team's activity (its messages and git actions) is shown in one optional panel beside the terminal, never in it (the CEO's decision, 2026-10-16: it replaces the thin marker strip along each terminal and the Messages drawer, which said little). The **`Activity`** toggle in the header opens it.

A right-hand column (**360px**, `--panel`, left border) for the selected deployment only ([Team message](../models/team-message.md)). It narrows the terminal rather than covering it.

- **Header:** "Activity" (600), subline `<Team> on <project> only` (12.5px `--tx4`), `×`.
- **Names are labels** everywhere in the panel, in the primary colour: a small box (`--accSoft`, 1px `--accLine`, radius 5, 12px 600, `--accTx`); the CEO's (`You`, `--raised`, `--tx`) and Foreman's (muted) stand apart.
- **An exchange reads `<agent> <verb> <agent>`:** the verb, in the past tense and its kind's colour, needs no box and no arrow (it says the direction): `asked` (`--info`), `replied to` (`--ok`), `handed off to` (`--tx2`), `wrote to` and `notified` (`--tx3`). Names keep their labels.
- **Who calls whom** (compact, 150px tall): a name box per agent on an ellipse (in the primary colour, filled `--acc` when it is the filter), with its status dot (and the word `queued` when it waits for a session slot: the dashed dot is too small to tell from idle; the box's tooltip names the status), plus the CEO when they sent something. Foreman is never on the map (the CEO, 2026-10-17): its notices (a merge done, a conflict, `main` moving) are system news, not teamwork, so they show in the list only. A line joins each pair that exchanged messages, thicker with more of them, `--line3`, or `--acc` while a message between them waits; at its middle a small count badge (Mono 10.5px) whose tooltip gives both directions. Clicking a name box shows only its messages; clicking a badge, only that pair's; again (or `Show all` in the strip that names the filter) shows everything.
- **The exchanges, newest activity first, one row per thread** (an ask and its replies; a hand-off, a contribution or a notice on its own): the exchange (`Tic asked Tac`), a spacer, how it stands (`no reply yet`, `1 reply`, `waiting` in `--accTx`, `expired`, `failed`), the time of its last message (local: `14:05` today, `Oct 15 14:05` before); under it the first message's start, on one line. A click opens the thread as a chat: each message a bubble (the asker's on the left, `--raised`; the other side's on the right, `--panel`), with its sender's label, `kind · msg-N`, the time, the whole text (selectable; its `inline code` and ``` fenced blocks ``` shown as code, the rest as plain text, never HTML), the artifact, `Waiting: <why> (the recipient's state: not started because every session slot is taken, idle agents included, with the count; busy until its turn ends; waiting for the CEO; asleep and being woken; the CEO's unsent line in its terminal), the row's `waiting` carrying the same as its tooltip` or a failure's reason; then `Open <asker>'s session`.

- **Git actions** (`switch`, `commit`, `merge`, `abort_merge`, `discard`, `set_submodule` (its commit message, which says what it did), and the lead's `request_merge`, which names each merge request it made and links **Review it**, which opens the project's Overview; [Protocol → Team actions](../architecture/protocol.md#team-actions)) sit between the threads, one row each, newest first, on a faint `--side` tint: the agent's label and what it did (`committed "cal v0"` for one repo, `committed api, project` for several, each repo's line then ending with its own message; `switched to wt/tac/LED-44 (new)`, `merged wt/tac/main`, `aborted a merge`; `tried to …` and `refused` in `--err` when refused), the time; under it each repo (Mono 11.5px: the repo, its branch, the commit's 7 characters, conflicts in `--accTx`), or the refusal's text. Filtering by an agent shows its actions; filtering by a pair leaves them out.

What an agent itself receives (a teammate's ask, a reply, the CEO's answer) is a **delivery**: typed into its session when it is idle, so it appears in the terminal as the harness shows any input. The texts are in [Protocol → Team tools](../architecture/protocol.md#team-tools).

**Later:** Foreman's other notes for the CEO (`instance.marker` events: escalations, answers, expiries, system notes, with the glyphs in [Team tools → Markers](../architecture/team-tools.md#markers-notes-for-the-ceo-never-in-the-terminal)) join the list as rows of their own.
