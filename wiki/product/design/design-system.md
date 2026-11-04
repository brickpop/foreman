---
type: spec
status: planned
tags: [design, ui]
---

# Design system

The visual language of the Foreman UI: tokens, type, spacing, surfaces and the small set of components every screen is built from. The visual reference is the clickable [prototype](./prototype/foreman-prototype.dc.html) (open it in a browser). It is reference only: its inline styles, simulated data and dead state are not production code. This entry is the source of truth when the two disagree: the prototype still draws agents' faces, which Foreman no longer has (the CEO, 2026-11-02; an agent shows as its name, with a status dot where its state matters).

Two themes, **dark** and **light** (the CEO picks one, or `System` to follow the operating system; see [Settings](../features/settings.md#appearance)), built on the same tokens. Since 2026-11-02 (the CEO: "too dark, and the yellow…"; functional yet fatigue-free) dark is GitHub's **dark dimmed** (slate greys, not black) and light is GitHub's **light** (cool neutral greys). The **accent** is the CEO's pick of seven (blue by default), and **attention** (what needs the CEO) keeps its own amber whatever the accent, so "needs you" never reads as a selection. Every color in the UI comes from a token, so a theme is a token set. The theme and the accent are the CEO's preferences (`~/.foreman/preferences.toml`, [Settings → Appearance](../features/settings.md#appearance)).

## Tokens

One set for both themes in `ui/src/styles.css`: each token is `light-dark(<light>, <dark>)`, picked by the page's `color-scheme`: `light dark` (the operating system's) under System, `dark` or `light` under `html[data-theme]`. `ui/src/styles.test.ts` checks that every text token, and every accent's text and ink, keeps 4.5:1 (WCAG AA) on its page and fill in both themes.

| Token | Dark | Light | Used for |
|---|---|---|---|
| `--bg` | `#22272E` | `#FFFFFF` | Page background, swatch inner ring |
| `--side` | `#1C2128` | `#F6F8FA` | Sidebar, agent-tab strip, board cells |
| `--panel` | `#2D333B` | `#FFFFFF` | Panels (request panels, lists, board cards), drawers, modals, toasts, callouts |
| `--raised` | `#373E47` | `#EAEEF2` | Selected nav/segment, raised buttons, chips, stage chips |
| `--head` | `#373E47` | `#F6F8FA` | A card's header band (GitHub's box header: a step lighter than the card in dark, a step darker in light) |
| `--hover` | `#323942` | `#F3F4F6` | Row and item hover |
| `--term` | `#1C2128` | `#F6F8FA` | Terminals, `<pre>`, inputs, textareas, CoS result panel |
| `--line` | `#373E47` | `#D8DEE4` | Default borders and dividers |
| `--line2` | `#444C56` | `#D0D7DE` | Button and input borders, dashed empty states, scrollbar thumb |
| `--line3` | `#545D68` | `#AFB8C1` | Hover border, selected segment border, toast border, unchecked box |
| `--tx` | `#CDD9E5` | `#1F2328` | Primary text |
| `--tx2` | `#ADBAC7` | `#424A53` | Body and secondary text |
| `--tx3` | `#96A2AF` | `#59636E` | Muted text, subtitles |
| `--tx4` | `#848F9C` | `#6E7781` | Section labels, meta, timestamps |
| `--acc` | [accent](#accents) | [accent](#accents) | The accent as a fill: primary buttons, toggles, active tab underline, selected option border, focus ring, the logo |
| `--accTx` | [accent](#accents) | [accent](#accents) | The accent as text: links, the `❯` prompt, the "lead" tag, the selected branch or file |
| `--accInk` | [accent](#accents) | [accent](#accents) | Text on an `--acc` fill |
| `--accSoft` | `--accTx` at 14% | same | Selected option background, "me" chat bubble (translucent: right on any surface) |
| `--accLine` | `--accTx` at 40% | same | Border of `--accSoft` surfaces, "lead" tag border |
| `--attn` | `#C69026` | `#BF8700` | Attention as a fill: needs-you and changed dots, the Inbox count badge, the awaiting status and ring, a waiting call on the map |
| `--attnTx` | `#DAAA3F` | `#9A6700` | Attention as text: approval kind, `needs you`, waiting and queued messages, uncommitted changes, medium priority, warnings |
| `--attnInk` | `#22272E` | `#1F2328` | Text on an `--attn` fill |
| `--attnSoft` / `--attnLine` | `--attnTx` at 14% / 40% | same | Warning boxes (a stale team file, a team replaced, a draft waiting) |
| `--ok` | `#57AB5A` | `#1A7F37` | Running, success, "token set" |
| `--info` | `#6CB6FF` | `#0550AE` | Done, question kind, info callout |
| `--team` | `#B083F0` | `#8250DF` | team-message markers, starting status, finding kind, `→` in Messages |
| `--err` | `#F47067` | `#CF222E` | Stopped, errors, blocked kind, high priority |
| `--shadow` | `0 12px 32px rgba(0,0,0,.45)` | `0 12px 32px rgba(140,149,159,.3)` | Drawers, modals, toasts |

The fixed palettes that only tell things apart (a tag's dot, a git graph's lanes) use `--attn`, `--info`, `--ok`, `--team`: never the accent, so they don't change with it.

### Accents

`html[data-accent]` picks one; the attribute works on any element, so a Settings swatch shows its own. Each is a fill, its text, and the accent as text (all white ink but amber's):

| Accent | `--acc` dark | `--accTx` dark | `--acc` light | `--accTx` light |
|---|---|---|---|---|
| **blue** (default) | `#316DCA` | `#539BF5` | `#0969DA` | `#0969DA` |
| green | `#347D39` | `#57AB5A` | `#1F883D` | `#1A7F37` |
| purple | `#8256D0` | `#B083F0` | `#8250DF` | `#8250DF` |
| teal | `#1D7A72` | `#4CC3B5` | `#1B7C83` | `#1B7C83` |
| orange | `#AE5622` | `#E0823D` | `#BC4C00` | `#BC4C00` |
| pink | `#AE4C82` | `#E275AD` | `#BF3989` | `#BF3989` |
| amber | `#C69026` | `#DAAA3F` | `#BF8700` | `#9A6700` (ink `#22272E` / `#1F2328`) |

Green, purple and amber share their hue with `--ok`, `--team` and attention: the CEO's pick.

Fixed values that are not tokens (keep them as named constants): modal backdrop `rgba(5,6,7,0.55)` in both themes; toggle knob `#fff`. The selection is `--acc` at 35%.

Global rules: `html, body` have no margin, `--bg` background and `--tx` color. Links are `--accTx`, with no underline, not even on hover: no clickable element shows an underline (the CEO's rule). Every clickable element (buttons, links, tabs, toggles) shows the pointer cursor; a disabled one shows `not-allowed`. WebKit scrollbars are 10px with a `--line2` thumb, radius 8px and a 2px `--bg` border; the agent-tab strip uses `scrollbar-width: thin`.

## Type

Fonts: **Geist** (400/500/600/700) for UI and **Geist Mono** (variable) for code, paths, branches, ids, chips and diffs. The terminals have their own font, **JetBrains Mono** by default, set in [Settings](../features/settings.md#appearance) (the CEO, 2026-10-21: JetBrains Mono suits a terminal, not the rest of the UI; the terminal font setting changes the terminals only). App root: Geist, `system-ui` fallback, 14px.

| Role | Size / weight / extras |
|---|---|
| Page H1 (Dashboard, Project, Team, Settings) | 30px / 600 / `letter-spacing: -0.02em` |
| Agent page H1 | 26px / 600 / -0.02em |
| Wiki H1 (reader) | 30px / 600 / -0.02em / line-height 1.2 |
| Wiki H1 (side sheet) | 22px / 600 |
| Wiki H2 | reader 18px / 600, margin-top 16px; sheet 15px / 600, margin-top 10px |
| Wiki paragraph | reader 15.5px / line-height 1.65 / `--tx2`; sheet 14.5px |
| Wiki list item, link, checkbox | 15px / line-height 1.6 |
| Dialog title | 20px / 600 |
| Page subtitle | 15px / `--tx3` |
| Section title ("Projects", "Needs you", "Waiting on the lead", …), a page's sections (the CEO, 2026-11-02: they read as titles) | 15px / 600 / `--tx`, its count after it when it helps (13px / 400 / `--tx4`) |
| Group label (a repo inside a review's tree, a kind of outdated comment) | 13px / 500 / `--tx4` |
| Brand | 15px / 600 / -0.01em |
| Nav item | 14px / 500 |
| Sidebar project row | 13.5px |
| Request text | Dashboard 16px, project Inbox (Needs you) 15.5px; line-height 1.5; `text-wrap: pretty` |
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
| 5px | Frontmatter chip (a tag or a list value, [Wiki reader](../features/wiki-reader.md#frontmatter)), stage chip |
| 6px | Nav items, small buttons, swatches, skill chips, settings inputs |
| 7px | Standard buttons |
| 8px | Board card, code block, textareas |
| 9px | Selectable rows (Add project), CoS input, agent "Running on" rows, small badge |
| 10px | Board cell, `<pre>` panels, callout, toast, CoS result panel, sidebar badge |
| 12px | Panels (request panels, lists), CoS bubbles, empty states |
| 14px | Modals |
| 16px | CoS quick-reply chips |
| 50% | Dots |

## Spacing and layout

- Page padding: one rule everywhere, **24px sides**, 24px top (20px on the Wiki and a review's header) and 48px bottom, `24px 24px 48px` (the CEO, 2026-10-31: compact and consistent; 48px sides until then). Width caps belong to text meant to be read, not to padding: the wiki entry and a project's description 780px, the Inbox, Team deployments and Members 920px, Settings and Playbook 760px, the Dashboard 1080px ([Project page → Layout](../features/project-page.md#layout)). Under 760px the Dashboard's sides are 16px.
- Max widths: Dashboard 1080px; Team Deployments and Members, the Inbox 920px; Settings 760px; Project settings' left column 520px; Playbook 760px; File 860px; the wiki article and a project's description 780px; `wiki.toml` panel 620px.
- Vertical gaps: 40px between Dashboard sections; 36px between Settings sections and between the Team section's parts; 28 / 24 / 20px elsewhere. List gaps 10px; inline gaps 6–14px.
- Row padding: list rows `14px 4px` or `16px 6px`; table-like rows `12px 16px`; settings rows `14px 0` or `12px 0` with a bottom border.

## Surfaces

- Default border: `1px solid var(--line)`. Inputs and buttons: `1px solid var(--line2)`.
- **Panel**: `--panel` background, `--line` border, radius 12. A list inside a panel separates rows with a `--line` bottom border and clips with `overflow: hidden`.
- **Card** (`Card`; Dashboard projects and teams, Repos, Project settings; the CEO, 2026-11-02: cards were dull): a panel in two parts. A **header band** (`--head`, `--line` bottom border, padding `8px 16px`, at least 44px) holds the name (600 15px, cut with an ellipsis) and, on the right, its **status pill** or its menu; the **body** (padding `14px 16px`, gap 12) holds the rest. A card that opens something gets a `--line3` border on hover, and its name's link covers all of it. A collapsed card (Project settings → How it works) is its band alone. A **danger** card (irreversible actions) has an `--err` border at 50%.
- **Status pill** (`StatusPill`): the status's 7px dot and word (12px `--tx2`) on a 16% tint of its color, fully rounded, padding `2px 8px`.
- **Loading** (`Loading`): where data is still on its way, faint bars (12px, `--hover`, pulsing unless the CEO's system asks for reduced motion) in a panel, in place of the content and never its empty state, which would read as "there's none". Every list, page and file view that waits on the server shows it (the CEO, 2026-10-21).
- **Empty state**: `1px dashed var(--line2)`, radius 12, `--tx3` text, centered copy.
- **Not built yet**: an empty state where the server doesn't serve an area yet: "**Not built yet.** The server doesn't serve <the wiki>." ("Not built yet." in `--tx2`, 500), in place of what the area would show.
- **Overlays** (drawers, side sheet, modals, toasts) use `--panel` and `--shadow`. Layering: side sheet 44, modals 50, toasts 60. See [App shell](../features/app-shell.md).

## Status colors

Instance status ([Instance](../models/instance.md)) maps to one color everywhere (dots, rings, text):

| Status | Color |
|---|---|
| `running` | `--ok` |
| `awaiting` (needs the CEO) | `--attn` |
| `idle` | `--tx4` |
| `done` | `--info` |
| `starting` | `--team` |
| `stopped` | `--err` |
| `queued` (waiting for a session slot) | `--tx4` dot drawn as a dashed outline (no fill), so it reads differently from `idle` |
| `asleep` (no session until it is woken) | `--tx4` dot drawn as a solid outline (no fill) |

Project state text (Dashboard): `needs you` `--attnTx`, `on track` `--ok`, `idle` `--tx4`.

## Chips and badges

| Element | Style |
|---|---|
| **Request kind chip** | Mono 600 10.5px, `.05em`, uppercase; color and 1px border in the kind color; radius 4; padding `1px 6px`; no wrap. Kinds: Approval `--attnTx`, Question `--info`, Review `--ok`, Blocked `--err`, Finding `--team` ([Requests](../features/requests.md)). In explanatory lists the same label appears without a border. |
| **Count badge** (the sidebar's Inbox) | `--attn` background, `--attnInk` text, Mono 600 10.5px, padding `1px 6px`, radius 9 |
| **Needs-you dot** | 7px `--attn` circle |
| **Changed dot** | 6px `--attn` circle, tooltip "Changed since you last looked" |
| **Frontmatter chip** | 12.5px, `--raised`, radius 5, padding `3px 8px`, `--tx3` key; the value in `--tx`. Format `key: value` |
| **Stage chip** | 12.5px, `--tx2` on `--raised`, radius 5, padding `3px 8px` |
| **"lead" tag** | 11.5px, `--accTx`, 1px `--accLine` border, radius 4, padding `0 5px`. Beside a lead's name, left out when the name says it already ("Lead", "Tech lead", "QA_lead": `lead` starting a word, any case; the CEO, 2026-11-02, after "Lead lead") |
| **Skill chip** | 13px, `--raised`, `--tx2`, padding `4px 6px 4px 10px`, radius 6, trailing `×` in `--tx4` (removes). The "+ add" input has a `1px dashed --line2` border and is 80px wide |
| **Quick-reply chip** (CoS) | `--raised`, 1px `--line2`, radius 16, padding `6px 12px`, 500 13px; hover border `--acc` |
| **Priority text** (board) | Mono 500 11.5px: quick `--ok` (no uncertainty, doable right now), high `--err`, medium `--attnTx`, low `--tx4`; any other value `--tx4` |

## Buttons

| Variant | Style | Used for |
|---|---|---|
| **Primary** | `--acc` background, no border (or 1px `--acc`), `--accInk`, 600 13px, radius 7, padding `7–8px 13–14px` | The main action of a view: Deploy, Add project, New team, Deploy to a project, Send to X, the first request option |
| **Secondary** (outline) | Transparent, 1px `--line2`, `--tx2` (or `--tx`), 500 13px, radius 7, padding `6–8px 12–14px` | Cancel, Hire, Deploy a team, Refine with Chief of Staff, the stepper |
| **Accent outline** | Transparent, 1px `--acc`, `--accTx`, 600 13px, radius 7; `--accSoft` on hover | A view's main action when it shouldn't outshine the content: Add a repo (with the repo icon), Start <Name> on an asleep or stopped agent (with a play icon); an icon, padding `8px 16px` (the CEO, 2026-11-04) |
| **Raised** | `--raised`, 1px `--line2`, `--tx` | The Files toggle while open (Wiki), a review comment's `Add to the draft` / `Save` |
| **Muted outline** | Outline with `--tx3` | `wiki.toml` |
| **Danger** | Transparent, 1px `--line2` border (`--err` on hover), `--err` text, 500 13px, radius 7 | Destructive actions: Stop deployment, Dismiss a review. Always behind a confirmation |
| **Ghost** | Transparent, transparent border, `--tx3` | "Open session" on the Dashboard approval request panels |
| **Segmented** | On: `--accSoft`, `--accTx`, 500 (the CEO, 2026-11-02: the one chosen stands out). Off: transparent, `--tx3` | Board switcher, the Team section's views, a review's change, Theme |
| **Option** | On: `--accSoft` background, `--acc` border. Off: transparent, `--line2` | Harness, prompt switcher, Deploy dialog choices |
| **Select** (`components/Select.tsx`, the one dropdown; the CEO, 2026-10-18) | A trigger like an input (`--panel`, 1px `--line2`, radius 7, 13px, the chosen label and a `▾`; `--acc` border while open), opening a menu like the Board's (`--panel`, `--line2`, radius 8, `--shadow`, at least the trigger's width, at most 360px tall, scrolling): one row per choice, `✓` beside the chosen one, a detail on the right in `--tx4`. Arrow keys and typing a letter move through it | The repo picker, a project's profile, a new profile's harness, the Podman machine's memory and CPUs. No native `<select>` |
| **Icon close** | Transparent, no border, `--tx3`, 16–18px `×` | Drawers, side sheet |
| **Small** (outline) | Transparent, 1px `--line2`, `--tx2`, 500 12.5px, radius 6, padding `4px 10px` | Actions inside a row or header: Session, Open PR ↗, Open in wiki, Sync, Connect, Edit, Rebuild |
| **Tiny** | 1px `--line2`, radius 5, padding `2px 8px`, 400 12px, `--tx3` | The draft box's Pause / Resume, a review comment's Edit and Delete |

**Request option buttons** follow their position: option 1 is primary (600); option 2 is outline with `--tx`; options 3+ are outline with `--tx2`. Size `7px 14px`, 13.5px on the Dashboard and 13px in the project's Inbox (Needs you).

## Form controls

- **Text input / textarea**: `--term` background, 1px `--line2`, radius 7–9, padding `7–10px 10–12px`, `--tx`. Textareas resize vertically.
- **Toggle switch**: track 34×20, radius 10, `--acc` on, `--line3` off. Knob 16×16 `#fff` at `top: 2px`, `left: 2px` (off) or `16px` (on), 0.15s transition.
- **Stepper**: two 28×28 outline buttons `−` and `+` around the value (Mono 500 14px, min-width 28px, centered).
- **Color swatch**: 22×22, radius 6. Selected ring `box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--tx)`.
- **Checkbox** (wiki): 16px, radius 4, 1.5px `--line3` border. Checked: the accent's fill (`accent-color: var(--acc)`), label `--tx3` with line-through.

## Hover

Rows and nav items get `--hover`. "+ New project" / "+ New team" get `--tx` on `--hover`. Board cards get a `--line3` border. "On this page" items turn `--tx`. Wiki links never underline (no underline anywhere). CoS chips get an `--acc` border.

## Accessibility (v1 scope)

The prototype has no focus styles, no Escape handling, no focus trap and fake disabled buttons (opacity 0.45 but still clickable). The real UI must:

- Use real `disabled` controls: not clickable, not focusable, 0.45 opacity, `not-allowed` cursor (as in the global rules).
- Show a visible focus ring on every interactive element for keyboard focus (`:focus-visible`): 2px `--acc` outline with a 2px offset.
- Close the topmost overlay (side sheet, drawer, modal) on **Escape**, and return focus to the element that opened it.
- Trap focus inside modals while they are open.
- Give icon-only buttons (`×`) an accessible label.

## Not to replicate from the prototype

The prototype carries simulation-only state that has no meaning in the real app: section chevrons and collapsible sidebar sections, a forge picker, a theme toggle helper, "needs" counters that disagree with each other, and in-memory edits to the project list. Behaviour is specified by the feature entries, not by the prototype's code.
