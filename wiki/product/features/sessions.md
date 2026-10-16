---
type: spec
status: planned
tags: [feature, ui, sessions, team-tools]
---

# Sessions

The terminal multiplexer. Every agent [instance](../models/instance.md) is a live harness process with a PTY ([Session runner](../architecture/session-runner.md)); this screen shows its real terminal in the browser, with keyboard input, copy and paste. It is also where the CEO talks to a team: through the lead by default, or to any member directly.

## Top bar

Padding `10px 16px`, bottom border.

- **Left: deployment tabs**, one per active [deployment](../models/deployment.md), labelled `<Team> · <project>` (segmented style). A tab whose deployment needs the CEO shows an amber needs-you dot. Selecting a tab shows that deployment's agents and loads the lead in a single pane. Each tab ends in a `×` (`--tx4`, `--tx` on hover, title `Stop`), beside the label rather than inside it: it **stops** that deployment, after the same confirmation as the [Team page](./team-page.md)'s Stop (one dialog, `StopDialog`). Once stopped, its tab is gone.
- **Right**: a **`Split`** / **`Single`** toggle (on when split) and a **`Messages`** toggle (on when open), both in the segmented style ([Design system](../design/design-system.md#buttons)).
- **Empty** (no deployments): centered on `--bg`, `No team is deployed yet, so there are no sessions.` and a primary **`Deploy a team`** button, which opens [Deploy a team](./deploy-team.md) with nothing chosen (the team, the project and the goal).

## Agent tabs

A strip (`--side`, padding `0 12px`) that scrolls horizontally only.

1. The **lead first**, with the "lead" tag.
2. A 1×20px `--line2` divider and the label **`Direct`** (12px `--tx4`).
3. The other members, in roster order.

Each tab: Avatar 24 with its status dot (`dotBorder --side`), the name (500). Underline (inset 2px bottom): `--acc` if the agent is in the active pane, `--line3` if it is in another pane, none otherwise. Text is `--tx` when shown in a pane, `--tx3` otherwise.

Clicking a tab: if the agent is already in a pane, that pane becomes active; otherwise the agent replaces the active pane's content.

## Panes

One pane, or **two** side by side (equal columns, 1px `--line` divider); two is the maximum in v1. The open panes are remembered per deployment and restored when it is opened again ([UI state](../models/ui-state.md)).

- **Split** adds the first member not in pane 1 as pane 2 and makes it active. **Single** keeps the active pane's agent.
- Clicking (mouse down) in a pane makes it active. When split, the active pane shows a 2px `--acc` top inset.

**Pane header** (padding `9px 16px`, 13px): when split, Avatar 26 with status dot (`dotBorder --term`) and the name (600, opens the [Agent page](./agent-page.md) showing this deployment's prompt); in a single pane both are left out, since the active tab already names the agent. Then `<Harness label> · <project>` (`--tx4`); a spacer; **`Line`** (toggles the line input below); **`Copy`** (copies the whole buffer, scrollback included, as plain text; shows `Copied` for 1.4 s); **`×`** when split (closes the pane; pane 1 becomes active).

**Asleep agent** (status `asleep`: no session runs until it has work, [Agent runtime → Lazy starts](../architecture/agent-runtime.md#lazy-starts)): the pane has no terminal, and its header no `Line` or `Copy`. Centered instead: `<Name> is asleep: no session runs until there's work for them.` (13.5px `--tx3`) and an outline **`Start <Name>`** button, which wakes it; the terminal appears once it runs. A failure shows an error toast.

**Direct-line banner** on every non-lead pane (padding `7px 16px`, 12.5px `--tx3`, `--side`, bottom border): `Direct line to <Name>. They'll tell <Lead>, the lead, if it matters.`

**Body**: a real terminal emulator on `--term`, Geist Mono 13px / 1.6.
- Full ANSI (colors, cursor movement, alternate screen), as the harness renders it.
- Scrollback is replayed from the session log when a pane opens ([Persistence](../architecture/persistence.md)); the live stream follows.
- Native text selection, copy and paste; clickable links.
- **Shift+Enter** types the harness's newline keystroke (a new line without sending), the same as in the line input: a terminal alone can't tell it from Enter.
- The pane size is propagated to the PTY on every resize. One PTY has one size, so when several panes or browser tabs show the same instance, **the most recently focused view sets it**; the others render that size (letterboxed or scrolled).
- Auto-scroll follows output while the view is within 160px of the bottom; scrolling up stops following.
- Keystrokes go straight to the PTY, raw, always ([API](../architecture/api.md), `WS /ws/pty/:instance`). Foreman doesn't interpret them.

**Messages wait on the CEO's draft** (the CEO's request, 2026-10-16). Text the CEO types straight into an agent's terminal and hasn't sent yet is a **draft**, and a delivery would land on it. So while one is there, that agent's messages wait, and a strip above the line input (`--side`, 12.5px `--tx2`) says `1 message waiting for your line: send or clear it. 0:42`, with a tiny **`Pause`** / **`Resume`** button. The countdown starts at 1:00 and runs only while the CEO isn't typing (5 s after the last key), so they aren't rushed; `(waits while you type)` says so, and Pause holds it until resumed. Sending the line lets the messages follow at the agent's next idle (or at once, if the Enter started nothing); clearing it (Ctrl+U, Ctrl+C, Backspace to empty) lets them through at once. At 0:00 they go after the draft, separated by a blank line, `---` and `The CEO's text above is an unfinished draft: don't act on it yet.` ([Team tools → Delivery](../architecture/team-tools.md#delivery)). Foreman tells a draft from other keys by what is typed: printable text makes it, Enter, Ctrl+C and Ctrl+U end it, arrows and other escape sequences change nothing. Accepted: text typed into a harness's own dialog counts as a draft too, and clearing the prompt with the harness's own keys (Claude Code's Esc Esc) isn't seen, so the messages wait for the countdown.

**Line input** (optional mode, for paste or mobile; padding `11px 18px`, top border): an amber `❯` (Mono 500 13px `--accTx`) and a borderless text box that grows with its lines (up to 8). Enter sends it through Foreman (`POST /api/instances/:id/line`), which types it the way it types a delivery and in the same queue, so the two never mix: each line, each line break as the harness's **newline keystroke** ([Harness adapters](../architecture/harness-adapters.md)) after a short pause, then Enter after a pause. Sent in one burst, the Enter reached Claude Code as part of a paste (a line break, not sending; found 2026-10-16). A refusal (the agent stopped) shows beside the box. Shift+Enter starts a new line; an empty input still sends Enter. The mode is switched on by a **`Line`** button in the pane header, or **Ctrl+Shift+L** (**Cmd+Shift+L** on a Mac), anywhere on the screen, for the active pane (caught before the terminal, so the agent never gets it; Ctrl+L alone, and on a Mac every Ctrl key, still reach the agent). Opening Line puts the keyboard in it. The shortcut from the terminal goes back into an open Line; from Line, it closes it. Escape or the button also close it. Closing puts the keyboard back in the terminal. Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [Team tools](../architecture/team-tools.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. It goes raw to the member's terminal under the direct-line banner; Foreman posts nothing to the lead. The member tells the lead itself when the instruction matters to the team, as its [session prompt](../architecture/session-prompt.md#talking-to-the-ceo) asks.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

## Markers beside the terminal

The terminal shows only what the agent's program prints. Anything Foreman wrote into it would be typed input to the agent ([Protocol → Deliveries and markers](../architecture/protocol.md#text-into-a-session-deliveries-and-markers)), so Foreman's notes for the CEO about the team's traffic are **markers**, shown beside the terminal: a thin strip along the pane (newest last, one line each, the glyph first) and the Messages drawer below. They come from `instance.marker` events. **Phasing:** until `instance.marker` events exist (the [messaging epic](../../backlog/epics/messaging-handoffs.md)), the strip shows the `⇄` team traffic only, built from the team messages.

| Glyph | Meaning | Example | Color |
|---|---|---|---|
| `⇄` | team-message traffic | `⇄ Mara → Rhea · ask · msg-41`, `⇄ Rhea → Mara · reply · msg-41`, `⇄ Tove handed off wt/tove/LED-44 to Mara` | `--team` |
| `⚠` | Escalation to the CEO | `⚠ Ilya escalated an approval: new dependency decimal.js` | `--ask` |
| `✓` | Success / resumed | `✓ CEO answered Ilya: approve` | `--ok` |
| `✗` | Error | `✗ Delivery to Rhea expired: deployment stopped` | `--err` |
| `ⓘ` | System note | `ⓘ Foreman restarted, output may be missing` | `--tx4` |

What an agent itself receives (a teammate's ask, a reply, the CEO's answer) is a **delivery**: typed into its session when it is idle, so it appears in the terminal as the harness shows any input. The texts are in [Protocol → Team tools](../architecture/protocol.md#team-tools). A workspace setting hides the marker strip ([Settings](./settings.md)); the Messages drawer always lists the traffic.

## Messages drawer

A right-hand column (**340px**, `--panel`, left border) showing the team-message traffic of the selected deployment only ([Team message](../models/team-message.md)). It sits beside the panes and narrows them (it pushes rather than overlays, so the terminals are never covered).

- Header: "Messages between agents" (600), subline `<Team> on <project> only` (12.5px `--tx4`), `×`.
- Rows, newest first (padding `12px 16px`): `From` (600) `→` (`--team`) `To` (600), the kind (Mono 11.5px `--tx4`: `ask`, `reply`, `handoff`), a spacer, the time `HH:MM` (12px `--tx4`), then the text (13px `--tx2`, line-height 1.45).
- Clicking a row opens the sender's session.
