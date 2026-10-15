---
type: task
status: done
priority: high
tags: [feature]
epic: full-ui-demo
---

# Demo client: seed data, simulated events, per-area swap

**Why:** every screen can be built and judged before its backend exists, on data that looks like the real thing (the prototype's), without lying about what is real.

Unblocked: [API types](./api-types.md) is done.

**Done when:** a `demoClient` implements `ForemanClient` from seed data taken from the [prototype](../product/design/prototype/foreman-prototype.dc.html) (projects ledger, relay, harbor; teams Atlas, Vesper, Orion, Sentinel; their agents, deployments, requests, pipeline, wiki pages and boards; see the features' specs for exact copy); simulates events (status changes, a new request, messages, a hand-off) on a timer; `ui/src/main.tsx` picks real or demo per area; everything served by the demo client shows a "demo data" badge.

- [x] seed data module (`ui/src/api/demo/seed.ts`), typed with the generated types: 3 projects, 4 teams (14 agents), 4 deployments, requests, messages, work items, merge requests, wikis with boards; where the prototype disagreed with the specs the specs won (each spot has a `spec:` comment), and its Scrum Masters are reviewers (the role was dropped)
- [x] simulated events through the same subscription the real client offers (status changes, a team message, a new request, a hand-off, every 20 s while someone listens); writes change the demo's memory and announce themselves as events; wiki writes are version-guarded like the server's
- [x] per-area composition in main.tsx (one line per area): every area demo for now so the demo world stays coherent; the terminal is the server's real demo session; an area switches to real when its screen is wired to the backend
- [x] the "demo data" badge (`DemoBadge`), on each screen for the areas it shows, and in the sidebar
- [x] tests: the demo client satisfies the interface, its writes and events, its version guards, copies not shared memory, the simulation runs only while someone listens

**Retired 2026-10-15:** the UI went live and the demo client, its seed and the badge were deleted ([Going live](./epics/full-ui-demo.md#going-live)); they stay in git history.

**Gaps the seed extraction found, for the screen cards to settle:** the format of a request's links (a branch, a wiki entry) and the UI routes they open; the values of a checks result (`pass`, `fail`, empty, running?); how the server renders callouts and checkboxes in an entry's HTML; whether a folder's `index.md` also appears among the tree's children (the seed: no, the folder has `hasIndex`); where a board card's title comes from (H1, a `title` field, or the H1 without the id).
