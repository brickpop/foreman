---
type: task
status: blocked
priority: medium
tags: [feature]
---

# Image builder and Settings → Maintenance

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** containers need the harness and the project's toolchain ([Agent images](../product/architecture/tech-stack.md#agent-images)).

Blocked by [Experiment A](./prove-a-podman-engine.md).

**Done when:** the `Images` interface ([Building blocks → 1b](../product/architecture/building-blocks.md)) builds the four-layer chain through the API (transparent layers as tags, input-hash tags, Refresh, Prune), the Direct builder is a no-op, and [Settings → Maintenance](../product/features/settings.md#maintenance) shows and drives it.

- [ ] base and harness Containerfiles embedded in the binary
- [ ] chain build, tag, cascade invalidation; build logs streamed
- [ ] Refresh (with optional prune), Prune, status
- [ ] the Maintenance section
