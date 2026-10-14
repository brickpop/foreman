---
type: task
status: blocked
priority: high
tags: [feature]
epic: full-ui-demo
---

# Demo client: seed data, simulated events, per-area swap

**Why:** every screen can be built and judged before its backend exists, on data that looks like the real thing (the prototype's), without lying about what is real.

Blocked by [API types](./api-types.md).

**Done when:** a `demoClient` implements `ForemanClient` from seed data taken from the [prototype](../product/design/prototype/foreman-prototype.dc.html) (projects ledger, relay, harbor; teams Atlas, Vesper, Orion, Sentinel; their agents, deployments, requests, pipeline, wiki pages and boards; see the features' specs for exact copy); simulates events (status changes, a new request, messages, a hand-off) on a timer; `ui/src/main.tsx` picks real or demo per area; everything served by the demo client shows a "demo data" badge.

- [ ] seed data module, typed with the generated types
- [ ] simulated events through the same event subscription the real client offers
- [ ] per-area composition in main.tsx (one line per area)
- [ ] the "demo data" badge component, and where it shows
- [ ] tests: the demo client satisfies the interface and its events arrive
