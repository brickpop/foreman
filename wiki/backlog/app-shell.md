---
type: task
status: done
priority: high
tags: [feature]
epic: full-ui-demo
---

# App shell and the design-system components

Unblocked: [API types](./api-types.md) is done.

**Why:** every screen sits in the shell ([App shell](../product/features/app-shell.md)) and uses the same components ([Design system](../product/design/design-system.md), [Avatar](../product/design/avatar.md)).

**Done when:** the sidebar (Home, Sessions, projects, teams, Chief of Staff tile, Settings), toasts, dialogs and drawers exist as components over the tokens, with the accessibility baseline (focus ring, Escape, focus trap, real disabled), and the router serves the screens' routes.

- [x] TanStack Router (a route per screen; typed `tab` and `entry` search params) + Query (one hook per query; server events invalidate what they touch, a reconnect refetches everything) wired to the client, real or demo per area in `main.tsx`
- [x] sidebar from live data: open-request badge, needs-you dots, where each team is deployed, the Chief of Staff tile, the Settings row with the port
- [x] primitives (`components/ui.tsx`, `Overlays.tsx`, `Toasts.tsx`): buttons (every variant, real disabled), kind chip, count badge, dots (queued dashed), field chip, lead tag, panel, empty state, segmented (Radix ToggleGroup), modal and drawer (Radix Dialog: focus trap, Escape), toasts (6.5 s, click goes somewhere)
- [x] Avatar with status dot and ring, every trait of the spec; placeholder screens naming the card that builds each
- [x] the one breakpoint: under ~760px the sidebar collapses behind a menu button and opens as an overlay
- [x] the CEO looks at it on the Mac (`just setup`, then `just dev`): approved 2026-10-15
