---
type: spec
status: planned
tags: [feature, ui, sessions, acp]
---

# Sessions

The terminal multiplexer. Every agent [instance](../models/instance.md) is a live harness process with a PTY ([Session runner](../architecture/session-runner.md)); this screen shows its real terminal in the browser, with keyboard input, copy and paste. It is also where the CEO talks to a team: through the lead by default, or to any member directly.

## Top bar

Padding `10px 16px`, bottom border.

- **Left: deployment tabs**, one per active [deployment](../models/deployment.md), labelled `<Team> · <project>` (segmented style). A tab whose deployment needs the CEO shows an amber needs-you dot. Selecting a tab shows that deployment's agents and loads the lead in a single pane.
- **Right**: a **`Split`** / **`Single`** toggle (raised when split) and a **`Messages`** toggle (raised when open).
- **Empty** (no deployments): centered on `--bg`, `No team is deployed yet, so there are no sessions.`

## Agent tabs

A strip (`--side`, padding `0 12px`) that scrolls horizontally only.

1. The **lead first**, with the "lead" tag.
2. A 1×20px `--line2` divider and the label **`Direct`** (12px `--tx4`).
3. The other members, in roster order.

Each tab: Avatar 24 with its status dot (`dotBorder --side`), the name (500). Underline (inset 2px bottom): `--acc` if the agent is in the active pane, `--line3` if it is in another pane, none otherwise. Text is `--tx` when shown in a pane, `--tx3` otherwise.

Clicking a tab: if the agent is already in a pane, that pane becomes active; otherwise the agent replaces the active pane's content.

## Panes

One pane, or **two** side by side (equal columns, 1px `--line` divider).

- **Split** adds the first member not in pane 1 as pane 2 and makes it active. **Single** keeps the active pane's agent.
- Clicking (mouse down) in a pane makes it active. When split, the active pane shows a 2px `--acc` top inset.

**Pane header** (padding `9px 16px`, 13px): Avatar 26 with status dot (`dotBorder --term`); the name (600, opens the [Agent page](./agent-page.md) showing this deployment's prompt); `<Harness label> · <project>` (`--tx4`); a spacer; **`Copy`** (copies the whole buffer, scrollback included, as plain text; shows `Copied` for 1.4 s); **`×`** when split (closes the pane; pane 1 becomes active).

**Direct-line banner** on every non-lead pane (padding `7px 16px`, 12.5px `--tx3`, `--side`, bottom border): `Direct line to <Name>. <Lead>, the lead, is kept in the loop.`

**Body**: a real terminal emulator on `--term`, Geist Mono 13px / 1.6.
- Full ANSI (colors, cursor movement, alternate screen), as the harness renders it.
- Scrollback is replayed from the session log when a pane opens ([Persistence](../architecture/persistence.md)); the live stream follows.
- Native text selection, copy and paste; clickable links.
- The pane size is propagated to the PTY on every resize. When two browser views show the same instance, see open questions.
- Auto-scroll follows output while the view is within 160px of the bottom; scrolling up stops following.
- Keystrokes go straight to the PTY by default ([API](../architecture/api.md), `WS /ws/pty/:instance`).

**Line input** (optional mode, for paste or mobile; padding `11px 18px`, top border): an amber `❯` (Mono 500 13px `--accTx`) and a borderless input; Enter sends the line ([API](../architecture/api.md), `POST /api/instances/:id/input`). Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [ACP](../architecture/acp.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. Foreman shows the direct-line banner and posts a note into the lead's session: `ⓘ CEO gave <Name> a direct instruction: <text>`.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

## Foreman-injected lines

Foreman writes marked lines into sessions; they appear in the terminal like any output. Format (the glyph starts the line):

| Glyph | Meaning | Example | Color |
|---|---|---|---|
| `❯` | CEO input echo | `❯ Add idempotency keys to POST /settle` | `--accTx` |
| `⇄` | ACP traffic | `⇄ Mara asked (req-41): …`, `⇄ Rhea replied to req-41: …`, `⇄ Tove handed off wt/tove/LED-44` | `--acp` |
| `⚠` | Escalation to the CEO | `⚠ escalate: … [y/N]` | `--ask` |
| `✓` | Success / resumed | `✓ CEO answered: <answer>. Continuing.` | `--ok` |
| `✗` | Error | `✗ No teammate "X" on <project>. Teammates: A, B` | `--err` |
| `ⓘ` | System note | `ⓘ CEO gave Tove a direct instruction: …` | `--tx4` |

The exact delivery rules are in [ACP](../architecture/acp.md). A workspace setting hides ACP (`⇄`) markers ([Settings](./settings.md)).

## Messages drawer

A right-hand column (**340px**, `--panel`, left border) showing the ACP traffic of the selected deployment only ([ACP message](../models/acp-message.md)). It sits beside the panes and narrows them.

- Header: "Messages between agents" (600), subline `<Team> on <project> only` (12.5px `--tx4`), `×`.
- Rows, newest first (padding `12px 16px`): `From` (600) `→` (`--acp`) `To` (600), the kind (Mono 11.5px `--tx4`: `ask`, `reply`, `handoff`), a spacer, the time `HH:MM` (12px `--tx4`), then the text (13px `--tx2`, line-height 1.45).
- Clicking a row opens the sender's session.

## Open questions

1. **Messages drawer: overlay or push.** PRD says "right drawer"; the prototype pushes the panes (inline column). Recommendation: push, so the terminals are never covered.
2. **Messages row click target.** The prototype opens the sender. Options: sender; recipient; both in split. Recommendation: sender (unchanged).
3. **Pane limit.** Max 2 panes? Recommendation: max 2 in v1. (Persisting panes is owned by [UI state](../models/ui-state.md#open-questions).)
4. **Shared PTY size.** One PTY has one size, but several panes or browser tabs may show the same instance at different sizes. Options: last writer wins; the smallest view wins (tmux style); the focused view wins. Recommendation: the most recently focused view sets the size.
5. **Typing in the terminal vs line mode.** When the terminal takes keystrokes over the WebSocket, CEO input bypasses Foreman as lines, so the direct-instruction note to the lead and the `❯` echo can't be produced reliably ([API](../architecture/api.md)). Options: only line-mode input counts as a "direct instruction" (raw keystrokes are just typing; raw mode is for the harness UI itself: menus, confirmations); or post the note on the first Enter after typed input in a non-lead session, with the line reconstructed from the keystroke buffer. Recommendations differ: line mode only (from this entry) vs reconstruct on Enter (from the API entry).
