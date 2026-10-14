---
type: task
status: blocked
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Image builder (phase 1: one image) and Settings → Maintenance

**Why:** containers need the harness and the tools to run a project's tests. Phase 1 builds **one image for every project** ([Agent images → Phasing](../product/architecture/tech-stack.md#agent-images)).

Blocked by [Experiment A](./prove-a-podman-engine.md).

**Done when:** the `Images` interface ([Building blocks → 1b](../product/architecture/building-blocks.md)) builds the one image through Podman's API from an embedded Containerfile, the Direct builder is a no-op, and [Settings → Maintenance](../product/features/settings.md#maintenance) shows its state, harness version and build log, with **Rebuild**.

- [ ] the Containerfile, embedded in the binary: start from [`pclaude`](https://github.com/brickpop/pclaude)'s Dockerfile (commit `7c9f9079`: `node:26-slim`, shell tools, Go, Bun, Deno, Foundry, the `wiki` CLI, Claude Code last, `ENTRYPOINT []`, `CMD ["claude"]`), adding `foreman-agent`
- [ ] build through the API on first use; tag by input hash; build log streamed
- [ ] Rebuild without cache (latest Claude Code)
- [ ] the phase 1 Maintenance section

The four-layer chain, overrides, Refresh with prune and Prune: [Full image chain](./image-chain.md).
