---
type: spec
status: planned
tags: [feature, ui, sessions, team-tools]
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

One pane, or **two** side by side (equal columns, 1px `--line` divider); two is the maximum in v1. The open panes are remembered per deployment and restored when it is opened again ([UI state](../models/ui-state.md)).

- **Split** adds the first member not in pane 1 as pane 2 and makes it active. **Single** keeps the active pane's agent.
- Clicking (mouse down) in a pane makes it active. When split, the active pane shows a 2px `--acc` top inset.

**Pane header** (padding `9px 16px`, 13px): Avatar 26 with status dot (`dotBorder --term`); the name (600, opens the [Agent page](./agent-page.md) showing this deployment's prompt); `<Harness label> · <project>` (`--tx4`); a spacer; **`Copy`** (copies the whole buffer, scrollback included, as plain text; shows `Copied` for 1.4 s); **`×`** when split (closes the pane; pane 1 becomes active).

**Direct-line banner** on every non-lead pane (padding `7px 16px`, 12.5px `--tx3`, `--side`, bottom border): `Direct line to <Name>. They'll tell <Lead>, the lead, if it matters.`

**Body**: a real terminal emulator on `--term`, Geist Mono 13px / 1.6.
- Full ANSI (colors, cursor movement, alternate screen), as the harness renders it.
- Scrollback is replayed from the session log when a pane opens ([Persistence](../architecture/persistence.md)); the live stream follows.
- Native text selection, copy and paste; clickable links.
- The pane size is propagated to the PTY on every resize. One PTY has one size, so when several panes or browser tabs show the same instance, **the most recently focused view sets it**; the others render that size (letterboxed or scrolled).
- Auto-scroll follows output while the view is within 160px of the bottom; scrolling up stops following.
- Keystrokes go straight to the PTY, raw, always ([API](../architecture/api.md), `WS /ws/pty/:instance`). Foreman doesn't interpret them.

**Line input** (optional mode, for paste or mobile; padding `11px 18px`, top border): an amber `❯` (Mono 500 13px `--accTx`) and a borderless input; Enter sends the line ([API](../architecture/api.md), `POST /api/instances/:id/input`) as the same raw bytes typing it would produce (the text, then Enter). Placeholders:

| Situation | Placeholder |
|---|---|
| Instance is `awaiting` | `Reply to <Name>…` |
| Lead | `Tell <Name> what you need. They’ll delegate.` |
| Other member | `Direct instruction to <Name>…` |

## Talking to the team

- Typing to the **lead** is a request; the lead delegates through [Team tools](../architecture/team-tools.md) (`ask`, `handoff`).
- Typing to a **member** is a direct instruction. It goes raw to the member's terminal under the direct-line banner; Foreman posts nothing to the lead. The member tells the lead itself when the instruction matters to the team, as its [session prompt](../architecture/session-prompt.md#talking-to-the-ceo) asks.
- Typing to an `awaiting` instance answers its open request ([Requests](./requests.md)).

## Foreman-injected lines

Foreman writes marked lines into sessions; they appear in the terminal like any output. Format (the glyph starts the line):

| Glyph | Meaning | Example | Color |
|---|---|---|---|
| `⇄` | team-message traffic | `⇄ Mara asked (req-41): …`, `⇄ Rhea replied to req-41: …`, `⇄ Tove handed off wt/tove/LED-44` | `--team` |
| `⚠` | Escalation to the CEO | `⚠ escalate: … [y/N]` | `--ask` |
| `✓` | Success / resumed | `✓ CEO answered: <answer>. Continuing.` | `--ok` |
| `✗` | Error | `✗ No teammate "X" on <project>. Teammates: A, B` | `--err` |
| `ⓘ` | System note | `ⓘ Foreman restarted, output may be missing` | `--tx4` |

The exact delivery rules are in [Team tools](../architecture/team-tools.md). A workspace setting hides team-message (`⇄`) markers ([Settings](./settings.md)).

## Messages drawer

A right-hand column (**340px**, `--panel`, left border) showing the team-message traffic of the selected deployment only ([Team message](../models/team-message.md)). It sits beside the panes and narrows them (it pushes rather than overlays, so the terminals are never covered).

- Header: "Messages between agents" (600), subline `<Team> on <project> only` (12.5px `--tx4`), `×`.
- Rows, newest first (padding `12px 16px`): `From` (600) `→` (`--team`) `To` (600), the kind (Mono 11.5px `--tx4`: `ask`, `reply`, `handoff`), a spacer, the time `HH:MM` (12px `--tx4`), then the text (13px `--tx2`, line-height 1.45).
- Clicking a row opens the sender's session.
