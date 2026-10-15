---
type: epic
status: done
order: 0
---

# Full UI on a demo client

**Goal:** the whole UI of the prototype, every screen and flow, built now. Whatever the server can't serve yet runs on a **demo client**: in-memory, seeded with the prototype's data, simulating events (status changes, new requests, messages), behind the same `ForemanClient` interface as the real client. As the backend lands, the real client replaces the demo one **per area**, one line each in `ui/src/main.tsx` (the UI's composition root, [Overview → In the UI](../../product/architecture/overview.md#in-the-ui)).

It runs **alongside [Prove](./prove.md)**: the CEO runs the experiments while the UI is built.

**Conditions (the CEO's):**
1. **API types first.** The wire types from [Protocol](../../product/architecture/protocol.md) and the [models](../../product/index.md#data-models) are Go structs, each in the package that owns it (decided: no separate `internal/api`); TypeScript is generated from them with `tygo` (`just types`; the generated files are committed; `just lint` fails when they are stale). The demo client returns exactly those types, so the UI can't drift from the backend.
2. **Demo data is visibly marked:** anything served by the demo client carries a "demo data" badge.
3. **The demo client lives only in the UI's composition root,** and is never presented as real.

**Usable when done:** the CEO can walk every screen and flow of [the journey](../../product/journey.md) in the browser, with the terminal real and everything else on demo data, and judge the product before the backend exists.

## Going live

**The CEO's decision, 2026-10-15:** one mode, no parallel demo and real worlds. The CEO first reviews the whole UI on the demo client (`just dev`); once the screens are approved, areas start going live.

**Done the same day, all at once:** the screens were approved, and every area went to the real client. The demo client, its seed and the "demo data" badge are deleted (in git history). An area the server doesn't serve yet (Add a project, the wiki and board, requests, team messages, the pipeline, forge accounts, the Chief of Staff) refuses with `NOT_BUILT` and its screen says "Not built yet" ([Overview → In the UI](../../product/architecture/overview.md#in-the-ui)), until its card lands.

**Exit criteria:** every screen of the [features](../../product/index.md#features) is navigable with the prototype's flows; the terminal is real; everything else is marked as demo data.
