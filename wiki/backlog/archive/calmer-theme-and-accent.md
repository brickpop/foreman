---
type: task
status: done
priority: medium
tags: [feature, ui, design]
epic: workflow
---

# A calmer theme, a chosen accent, the Dashboard behind the brand

**Why:** "UI theme is a bit unpleasant. Too dark, and the yellow… I want a functional yet fatigue-free UI theme", with a configurable accent; and "Dashboard may not be needed as a menu entry. The top left icon + text should be the way to go to the dashboard page" (the CEO, 2026-11-02).

## Decided (the CEO, 2026-11-02)

- Dark becomes GitHub's **dark dimmed**, light GitHub's **light** ([Design system → Tokens](../../product/design/design-system.md#tokens)).
- The accent is one of seven **preset swatches** (no free color: each is tuned to stay readable), **blue** by default ([Settings → Appearance](../../product/features/settings.md#appearance)).
- **Attention keeps its own amber** (`--attn`): needs-you, awaiting, approvals, the Inbox badge, changed dots, waiting messages, warnings. The accent is only for actions, selection, links and focus.
- The sidebar's `Dashboard` item goes; the brand (logo and "Foreman") links to it ([App shell](../../product/features/app-shell.md)).

## Done

- [x] `styles.css`: one token set, `light-dark()` over `color-scheme` (the three copies of the light theme gone); accents as `[data-accent]` blocks usable on any element; soft shades translucent (`color-mix`); `--ask` (unused) gone
- [x] Every amber use sorted into accent or attention; the fixed palettes (tag dots, git lanes) on attention, so they don't move with the accent
- [x] `accent` preference (Go `userdata`, validated, a bad hand edit reads as blue); one `useSavePreferences` for the theme, accent and terminal font; the first frame painted from a browser copy
- [x] The terminal resolves its colors (`resolvedColor`) and follows theme and accent changes live (it kept its first colors before)
- [x] Tests: contrast of every text token and every accent in both themes (`styles.test.ts`, WCAG AA), the CSS accents matching the settings' list; accent preference kept, refused and defaulted
- [x] Specs: Design system, Settings, App shell, UI state, API, and every screen naming amber

## Left out

- The terminal's 16 ANSI colors are xterm's defaults, made for a dark background: on the light theme a harness's own colors can read poorly (the harness picks its own theme). As before this change.
