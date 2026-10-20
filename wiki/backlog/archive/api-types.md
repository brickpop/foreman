---
type: task
status: done
priority: high
tags: [feature]
epic: full-ui-demo
---

# API types: Go structs and generated TypeScript

**Why:** the UI and the server must share one definition of every type that crosses the wire, before any screen is built on them (the epic's first condition; [Tech stack → Wire formats](../../product/architecture/tech-stack.md#wire-formats)).

**Done when:** the wire types of [Protocol](../../product/architecture/protocol.md) and the [models](../../product/index.md#data-models) the UI reads (workspace, project, team, agent soul, deployment, instance, request, team message, work item, merge request, events) are Go structs with JSON tags, each in the package that owns it (no separate `internal/api`); `just types` generates `ui/src/api/types/<package>.ts` with `tygo` (listed in `tygo.yaml`); the client uses them.

Blocks every other card of this epic.

- [x] the Go structs, with JSON tags and comments that carry into TypeScript: workspace config, events, harness status, the `state.db` rows (deployments, instances, requests, team messages, work items, merge requests: `internal/store/types.go`) and the wiki's tree, entry and board (`internal/wiki/types.go`)
- [x] `tygo` pinned as a Go tool; `just types` recipe; generated files committed; `just lint` fails when they are stale (done with [Workspace and config](./workspace-config-events.md): workspace config and events)
- [x] `ForemanClient` extended to every area, typed with the generated types, and **grouped by area** (`workspace`, `agents`, `harnesses`, `requests`, `messages`, `pipeline`, `wiki`, `events`), each with a `demo` flag, so `main.tsx` can pick real or demo per area
- [x] the real client keeps working for workspace, instances and terminals (its other areas call the endpoints of [API](../../product/architecture/api.md), which land with their backend)
