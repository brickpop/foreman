---
type: task
status: blocked
priority: high
tags: [feature]
---

# App shell and the design-system components

Epic: [Full UI on a demo client](./epics/full-ui-demo.md)

Blocked by [API types](./api-types.md).

**Why:** every screen sits in the shell ([App shell](../product/features/app-shell.md)) and uses the same components ([Design system](../product/design/design-system.md), [Avatar](../product/design/avatar.md)).

**Done when:** the sidebar (Home, Sessions, projects, teams, Chief of Staff tile, Settings), toasts, dialogs and drawers exist as components over the tokens, with the accessibility baseline (focus ring, Escape, focus trap, real disabled), and the router serves the screens' routes.

- [ ] TanStack Router + Query wired to the client interface (real or demo per area)
- [ ] sidebar from live workspace data
- [ ] primitives: buttons, chips, segmented controls, panels, dialogs, drawers, toasts (Radix underneath)
- [ ] Avatar component with status dot and ring
