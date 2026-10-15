---
type: spec
status: planned
tags: [design, ui]
---

# Design system

The visual language of the Foreman UI: tokens, type, spacing, surfaces and the small set of components every screen is built from. The visual reference is the clickable [prototype](./prototype/foreman-prototype.dc.html) (open it in a browser). It is reference only: its inline styles, simulated data and dead state are not production code. This entry is the source of truth when the two disagree. The [Avatar](./avatar.md) has its own entry.

Two themes, **dark** and **light** (the CEO picks one, or `System` to follow the operating system; see [Settings](../features/settings.md#appearance)), built on the same tokens, with one accent: **amber**. Every color in the UI comes from a token, so a theme is a token set. The theme choice is a [workspace setting](../features/settings.md).

## Tokens

Dark is `:root`; light overrides it under `html[data-theme="light"]`.

| Token | Dark | Light | Used for |
|---|---|---|---|
| `--bg` | `#0E0F11` | `#F5F3EE` | Page background, default avatar dot border, swatch inner ring |
| `--side` | `#0B0C0E` | `#EDEAE3` | Sidebar, agent-tab strip, board cells, direct-line banner |
| `--panel` | `#131518` | `#FFFFFF` | Panels (request panels, lists, board cards), drawers, modals, toasts, callouts |
| `--raised` | `#1B1E22` | `#EEEBE4` | Selected nav/segment, raised buttons, chips, stage chips, Chief of Staff bubble |
| `--hover` | `#17191C` | `#E6E2DA` | Row and item hover |
| `--term` | `#08090A` | `#FBFAF7` | Terminal panes, `<pre>`, inputs, textareas, CoS result panel |
| `--line` | `#1F2227` | `#E0DCD3` | Default borders and dividers |
| `--line2` | `#2C3036` | `#CFCAC0` | Button and input borders, dashed empty states, scrollbar thumb |
| `--line3` | `#3A3F46` | `#B5AFA3` | Hover border, selected segment border, toast border, unchecked box |
| `--tx` | `#E6E3DC` | `#1B1A17` | Primary text |
| `--tx2` | `#C9C6BF` | `#3F3D38` | Body and secondary text |
| `--tx3` | `#9A9CA3` | `#5F5D57` | Muted text, subtitles |
| `--tx4` | `#80838A` | `#75726B` | Section labels, meta, timestamps |
| `--acc` | `#E8A33D` | `#E8A33D` | Amber accent: fills, active underline, dots |
| `--accTx` | `#E8A33D` | `#8A5200` | Accent used as text: links, the `❯` prompt, the "lead" tag |
| `--accInk` | `#17130B` | `#17130B` | Text on an amber fill |
| `--accSoft` | `#1A1610` | `#FBF1DF` | Selected option background, "me" chat bubble, CoS sidebar tile |
| `--accLine` | `#3A3020` | `#EBCF9E` | Border of `--accSoft` surfaces, "lead" tag border |
| `--ok` | `#6FCF8E` | `#23824A` | Running, success, "token set" |
| `--info` | `#7AA7E8` | `#2B66B5` | Done, question kind, info callout |
| `--team` | `#B79CF0` | `#7445C2` | team-message markers, starting status, finding kind, `→` in Messages |
| `--err` | `#E5675A` | `#B8352A` | Stopped, errors, blocked kind, high priority |
| `--ask` | `#F2BE6B` | `#8A5200` | Escalation (`⚠`) lines in the terminal |
| `--shadow` | `0 16px 40px rgba(0,0,0,.55)` | `0 16px 40px rgba(40,30,10,.18)` | Drawers, modals, toasts |

Fixed values that are not tokens (keep them as named constants): modal backdrop `rgba(5,6,7,0.55)` in both themes; toggle knob `#fff`; selection `rgba(232,163,61,.35)`.

Global rules: `html, body` have no margin, `--bg` background and `--tx` color. Links are `--accTx`, with no underline, not even on hover: no clickable element shows an underline (the CEO's rule). Every clickable element (buttons, links, tabs, toggles) shows the pointer cursor; a disabled one shows `not-allowed`. WebKit scrollbars are 10px with a `--line2` thumb, radius 8px and a 2px `--bg` border; the agent-tab strip uses `scrollbar-width: thin`.

## Type

Fonts: **Geist** (400/500/600/700) for UI and **Geist Mono** (400/500/600) for code, paths, branches, ids and chips. App root: Geist, `system-ui` fallback, 14px.

| Role | Size / weight / extras |
|---|---|
| Page H1 (Home, Project, Team, Settings) | 30px / 600 / `letter-spacing: -0.02em` |
| Agent page H1 | 26px / 600 / -0.02em |
| Wiki H1 (reader) | 30px / 600 / -0.02em / line-height 1.2 |
| Wiki H1 (side sheet) | 22px / 600 |
| Wiki H2 | reader 18px / 600, margin-top 16px; sheet 15px / 600, margin-top 10px |
| Wiki paragraph | reader 15.5px / line-height 1.65 / `--tx2`; sheet 14.5px |
| Wiki list item, link, checkbox | 15px / line-height 1.6 |
| Dialog title | 20px / 600 |
| Page subtitle | 15px / `--tx3` |
| Section label ("Projects", "Needs you", …) | 13px / 500 / `--tx4` |
| Sidebar section label | 12px / 500 / `--tx4`, padding `4px 10px 6px` |
| Brand | 15px / 600 / -0.01em |
| Nav item | 14px / 500 |
| Sidebar project / team row | 13.5px |
| Request text | Home 16px, Changes 15.5px; line-height 1.5; `text-wrap: pretty` |
| Kind chip | Mono 600 10.5px, `letter-spacing: .05em`, uppercase |
| Meta mono (paths, branches) | Mono 400 11.5–12.5px, `--tx4` or `--tx3` |
| Terminal | Mono 400 13px / line-height 1.6 |
| `<pre>` (YAML, prompt, code) | Mono 400 12.5px / line-height 1.6–1.65 |
| Folder-tree `<pre>` | Mono 400 13px / line-height 1.7, `white-space: pre` |
| Callout label | 11.5px / 600 / .05em / uppercase |

## Radii

| Radius | Used on |
|---|---|
| 4px | Kind chip, "lead" tag, checkbox |
| 5px | Frontmatter chip, stage chip, pane-header buttons |
| 6px | Nav items, small buttons, swatches, skill chips, settings inputs |
| 7px | Standard buttons |
| 8px | Board card, code block, textareas |
| 9px | Selectable rows (Add project), CoS input, agent "Running on" rows, small badge |
| 10px | Board cell, `<pre>` panels, callout, toast, CoS result panel, sidebar badge |
| 12px | Panels (request panels, lists), CoS bubbles, empty states |
| 14px | Modals |
| 16px | CoS quick-reply chips |
| 50% | Dots |

Avatar radius is `round(size × 0.24)` ([Avatar](./avatar.md)).

## Spacing and layout

- Page padding: Home, Settings `48px 48px 64px`; Project and Team header `40px 48px 0`; Agent `40px 48px 64px`.
- Project tab bodies: Overview `24px 32px 64px`; Board `20px 40px 64px`; Changes `28px 40px 64px`; Settings, teams part `24px 40px 64px`, config part `28px 48px 64px`.
- Team tab bodies: Deployments `24px 48px 64px`; Members `20px 48px 64px`; Playbook and File `28px 48px 64px`.
- Max widths: Home and Team tabs 920px; Settings 760px; Changes 860px; Project settings 960px; Playbook 760px; File 860px; wiki article: the full width of its column (narrower only by the Files tree); `wiki.toml` panel 620px.
- Vertical gaps: 40px between Home sections; 36px between Settings and Changes sections; 28 / 24 / 20px elsewhere. List gaps 10px; inline gaps 6–14px.
- Row padding: list rows `14px 4px` or `16px 6px`; table-like rows `12px 16px`; settings rows `14px 0` or `12px 0` with a bottom border.

## Surfaces

- Default border: `1px solid var(--line)`. Inputs and buttons: `1px solid var(--line2)`.
- **Panel**: `--panel` background, `--line` border, radius 12. A list inside a panel separates rows with a `--line` bottom border and clips with `overflow: hidden`.
- **Empty state**: `1px dashed var(--line2)`, radius 12, `--tx3` text, centered copy.
- **Overlays** (drawers, side sheet, modals, toasts) use `--panel` and `--shadow`. Layering: side sheet 44, Chief of Staff drawer 45, modals 50, toasts 60. See [App shell](../features/app-shell.md).

## Status colors

Instance status ([Instance](../models/instance.md)) maps to one color everywhere (dots, rings, text):

| Status | Color |
|---|---|
| `running` | `--ok` |
| `awaiting` (needs the CEO) | `--acc` |
| `idle` | `--tx4` |
| `done` | `--info` |
| `starting` | `--team` |
| `stopped` | `--err` |
| `queued` (waiting for a session slot) | `--tx4` dot drawn as a dashed outline (no fill), so it reads differently from `idle` |

Project state text (Home): `needs you` `--accTx`, `on track` `--ok`, `idle` `--tx4`.

## Chips and badges

| Element | Style |
|---|---|
| **Request kind chip** | Mono 600 10.5px, `.05em`, uppercase; color and 1px border in the kind color; radius 4; padding `1px 6px`; no wrap. Kinds: Approval `--accTx`, Question `--info`, Review `--ok`, Blocked `--err`, Finding `--team` ([Requests](../features/requests.md)). In explanatory lists the same label appears without a border. |
| **Count badge** (sidebar) | `--acc` background, `--accInk` text, Mono 600 11px, padding `1px 7px`, radius 10 |
| **Count badge** (tab) | Mono 600 10.5px, padding `1px 6px`, radius 9 |
| **Needs-you dot** | 7px `--acc` circle |
| **Changed dot** | 6px `--acc` circle, tooltip "Changed since you last looked" |
| **Frontmatter chip** | 12.5px, `--raised`, radius 5, padding `3px 8px`, `--tx3` key; the value in `--tx`. Format `key: value` |
| **Stage chip** | 12.5px, `--tx2` on `--raised`, radius 5, padding `3px 8px` |
| **"lead" tag** | 11.5px, `--accTx`, 1px `--accLine` border, radius 4, padding `0 5px` |
| **Skill chip** | 13px, `--raised`, `--tx2`, padding `4px 6px 4px 10px`, radius 6, trailing `×` in `--tx4` (removes). The "+ add" input has a `1px dashed --line2` border and is 80px wide |
| **Quick-reply chip** (CoS) | `--raised`, 1px `--line2`, radius 16, padding `6px 12px`, 500 13px; hover border `--acc` |
| **Priority text** (board) | Mono 500 11.5px: high `--err`, medium `--accTx`, low `--tx4` |

## Buttons

| Variant | Style | Used for |
|---|---|---|
| **Primary** | `--acc` background, no border (or 1px `--acc`), `--accInk`, 600 13px, radius 7, padding `7–8px 13–14px` | The main action of a view: Deploy, Add project, New team, Deploy to a project, Send to X, the first request option |
| **Secondary** (outline) | Transparent, 1px `--line2`, `--tx2` (or `--tx`), 500 13px, radius 7, padding `6–8px 12–14px` | Cancel, Hire, Deploy a team, Refine with Chief of Staff, the stepper |
| **Raised** | `--raised`, 1px `--line2`, `--tx` | Open sessions, Sessions |
| **Muted outline** | Outline with `--tx3` | `wiki.toml` |
| **Danger** | Transparent, 1px `--err` border, `--err` text, 500 13px, radius 7 | Destructive actions: Stop deployment, Reject. Always behind a confirmation |
| **Ghost** | Transparent, transparent border, `--tx3` | "Open session" on Home approval request panels |
| **Segmented** | On: `--raised`, `--line2` border (`--line3` for board and deployment tabs), `--tx`. Off: transparent, `--tx3` | Board switcher, deployment tabs, Listen on, Theme; and the on/off toggles Split and Messages, which look the same |
| **Option** | On: `--accSoft` background, `--acc` border. Off: transparent, `--line2` | Harness, avatar traits, prompt switcher, Deploy dialog choices |
| **Icon close** | Transparent, no border, `--tx3`, 16–18px `×` | Drawers, side sheet |
| **Small** (outline) | Transparent, 1px `--line2`, `--tx2`, 500 12.5px, radius 6, padding `4px 10px` | Actions inside a row or header: Session, Open PR ↗, Open in wiki, Sync, Change with Chief of Staff, Connect, Edit, Rebuild |
| **Tiny** | 1px `--line2`, radius 5, padding `2px 8px`, 400 12px, `--tx3` | Terminal pane header only: Line, Copy, × |

**Request option buttons** follow their position: option 1 is primary (600); option 2 is outline with `--tx`; options 3+ are outline with `--tx2`. Size `7px 14px`, 13.5px on Home and 13px on Changes.

## Form controls

- **Text input / textarea**: `--term` background, 1px `--line2`, radius 7–9, padding `7–10px 10–12px`, `--tx`. Textareas resize vertically.
- **Toggle switch**: track 34×20, radius 10, `--acc` on, `--line3` off. Knob 16×16 `#fff` at `top: 2px`, `left: 2px` (off) or `16px` (on), 0.15s transition.
- **Stepper**: two 28×28 outline buttons `−` and `+` around the value (Mono 500 14px, min-width 28px, centered).
- **Color swatch**: 22×22, radius 6. Selected ring `box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--tx)`.
- **Checkbox** (wiki): 16px, radius 4, 1.5px `--line3` border. Checked: amber fill, `✓` in `--accInk`, label `--tx3` with line-through.

## Hover

Rows and nav items get `--hover`. "+ New project" / "+ New team" get `--tx` on `--hover`. Board cards get a `--line3` border. "On this page" items turn `--tx`. Wiki links never underline (no underline anywhere). CoS chips get an `--acc` border.

## Accessibility (v1 scope)

The prototype has no focus styles, no Escape handling, no focus trap and fake disabled buttons (opacity 0.45 but still clickable). The real UI must:

- Use real `disabled` controls: not clickable, not focusable, 0.45 opacity, `not-allowed` cursor (as in the global rules).
- Show a visible focus ring on every interactive element for keyboard focus (`:focus-visible`): 2px `--acc` outline with a 2px offset.
- Close the topmost overlay (side sheet, drawer, modal) on **Escape**, and return focus to the element that opened it.
- Trap focus inside modals while they are open.
- Give icon-only buttons (`×`, avatar-only tabs) an accessible label.

## Not to replicate from the prototype

The prototype carries simulation-only state that has no meaning in the real app: section chevrons and collapsible sidebar sections, a forge picker, a theme toggle helper, "needs" counters that disagree with each other, and in-memory edits to the project list. Behaviour is specified by the feature entries, not by the prototype's code.
