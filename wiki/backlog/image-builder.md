---
type: task
status: todo
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Image builder (phase 1: one image) and Settings → Maintenance

**Why:** containers need the harness and the tools to run a project's tests. Phase 1 builds **one image for every project** ([Agent images → Phasing](../product/architecture/tech-stack.md#agent-images)).

Was blocked by [Experiment A](./prove-a-podman-engine.md), proven on macOS on 2026-10-14: its results are the starting point.

**Done when:** the `Images` interface ([Building blocks → 1b](../product/architecture/building-blocks.md)) builds the one image through Podman's API from an embedded Containerfile, the Direct builder is a no-op, and [Settings → Maintenance](../product/features/settings.md#maintenance) shows its state, harness version and build log, with **Rebuild**.

- [x] the Containerfile: `images/Containerfile`, a basic Debian box (`node:26-slim`: Node, Bun, Go, `just`, the `wiki` CLI, shell tools) with Claude Code last, adapted from [`pclaude`](https://github.com/brickpop/pclaude); built through the API by `just prove-image` on macOS (2026-10-14)
- [ ] embed it in the binary and add `foreman-agent`
- [ ] build through the API on first use; tag by input hash; build log streamed
- [ ] Rebuild without cache (latest Claude Code)
- [ ] the phase 1 Maintenance section

The four-layer chain, overrides, Refresh with prune and Prune: [Full image chain](./image-chain.md).
