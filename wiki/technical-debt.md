---
type: note
tags: [architecture]
---

# Technical debt

Workarounds and accepted limitations to revisit. Each item says why it exists and how it would be fixed. When one is scheduled, it becomes a card in the [backlog](./backlog/index.md) that links back here. When it's fixed, remove it from this page.

## `--demo-session`: one session started by a flag

- **State:** until deployments exist, nothing creates instances. The development flag `foreman --demo-session "<command>"` (used by `just dev`) starts one session, instance `demo`, through the supervisor. The command is split on spaces, with no quoting. The UI's Sessions screen lists whatever sessions exist.
- **Why:** a live session in the browser was needed before deployments, teams and instances exist.
- **Fix:** remove the flag and its wiring in `cmd/foreman` when [deployments](./product/models/deployment.md) start sessions ([Instance manager](./backlog/instance-manager.md)); `just dev` then deploys a test team instead.
