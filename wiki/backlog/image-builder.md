---
type: task
status: done
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Image builder (phase 1: one image) and Settings → Maintenance

**Why:** containers need the harness and the tools to run a project's tests. Phase 1 builds **one image for every project** ([Agent images → Phasing](../product/architecture/tech-stack.md#agent-images)).

Was blocked by [Experiment A](./prove-a-podman-engine.md), proven on macOS on 2026-10-14: its results are the starting point.

**Done when:** the `Images` interface ([Building blocks → 1b](../product/architecture/building-blocks.md)) builds the one image through Podman's API from an embedded Containerfile, the Direct builder is a no-op, and [Settings → Maintenance](../product/features/settings.md#maintenance) shows its state, harness version and build log, with **Rebuild**.

- [x] the Containerfile: `images/Containerfile`, a basic Debian box (`node:26-slim`: Node, Bun, Go, `just`, the `wiki` CLI, shell tools) with Claude Code last, adapted from [`pclaude`](https://github.com/brickpop/pclaude); built through the API by `just prove-image` on macOS (2026-10-14)
- [x] embed it in the binary (`images/images.go`); `foreman-agent` was to join the image with the agent link; it is mounted instead (decided 2026-10-15) ([Messaging and hand-offs](./epics/messaging-handoffs.md))
- [x] build through the API on first use (`Ensure`; concurrent callers share one build); tagged by a hash of the Containerfile, plus `:latest`; labelled `foreman.image`; build log kept (its last 256 KB) for Maintenance
- [x] Rebuild without cache (`Refresh`: no cache, latest base image, so the latest Claude Code); the Direct builder is a no-op
- [x] `just test-podman` on the CEO's Mac (2026-10-14): builds once for concurrent callers, a changed Containerfile is a new image, a failed build keeps its log, Refresh skips the cache

The four-layer chain, overrides, Refresh with prune and Prune: [Full image chain](./image-chain.md).
