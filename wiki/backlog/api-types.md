---
type: task
status: todo
priority: high
tags: [feature]
epic: full-ui-demo
---

# API types: Go structs and generated TypeScript

**Why:** the UI and the server must share one definition of every type that crosses the wire, before any screen is built on them (the epic's first condition; [Tech stack → Wire formats](../product/architecture/tech-stack.md#wire-formats)).

**Done when:** the wire types of [Protocol](../product/architecture/protocol.md) and the [models](../product/index.md#data-models) the UI reads (workspace, project, team, agent soul, deployment, instance, request, team message, work item, merge request, events) are Go structs with JSON tags, each in the package that owns it (no separate `internal/api`); `just types` generates `ui/src/api/types/<package>.ts` with `tygo` (listed in `tygo.yaml`); the client uses them.

Blocks every other card of this epic.

- [ ] the Go structs, with JSON tags and comments that carry into TypeScript
- [x] `tygo` pinned as a Go tool; `just types` recipe; generated files committed; `just lint` fails when they are stale (done with [Workspace and config](./workspace-config-events.md): workspace config and events)
- [ ] `ForemanClient` extended to every area, typed with the generated types
- [ ] the real client keeps working for workspace, instances and terminals
