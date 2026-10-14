---
type: task
status: blocked
priority: high
tags: [feature]
---

# Podman execution engine

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** real agents run in rootless containers ([Session runner](../product/architecture/session-runner.md)).

Blocked by [Experiment A](./prove-a-podman-engine.md).

**Done when:** `runner/podman` implements `Runner` and `Process` like `runner/direct` (start with mounts, env, labels and size; attach; resize; stop; wait; list for re-attach), with the Podman service started when needed, and integration tests behind a build tag that the CEO runs on macOS and Fedora.

- [ ] libpod REST client over the socket (`net/http`), attach as a hijacked stream
- [ ] mounts at the same path, `keep-id`, `:z` on SELinux
- [ ] Podman service start (`podman system service` / `podman machine start`), state in Settings → Maintenance
- [ ] the `engine: podman | direct` setting ([Workspace settings](../product/models/workspace-settings.md))
