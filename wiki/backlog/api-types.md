---
type: task
status: todo
priority: high
tags: [feature]
---

# API types: Go structs and generated TypeScript

Epic: [Full UI on a demo client](./epics/full-ui-demo.md)

**Why:** the UI and the server must share one definition of every type that crosses the wire, before any screen is built on them (the epic's first condition; [Tech stack → Wire formats](../product/architecture/tech-stack.md#wire-formats)).

**Done when:** `internal/api` holds the wire types of [Protocol](../product/architecture/protocol.md) and the [models](../product/index.md#data-models) the UI reads (workspace, project, team, agent soul, deployment, instance, request, team message, work item, pull request, events); `just types` generates `ui/src/api/types.ts` with `tygo`; the existing client uses them.

Blocks every other card of this epic.

- [ ] the Go structs, with JSON tags and comments that carry into TypeScript
- [ ] `tygo` pinned as a Go tool; `just types` recipe; generated file committed
- [ ] `ForemanClient` extended to every area, typed with the generated types
- [ ] the real client keeps working for workspace, instances and terminals
