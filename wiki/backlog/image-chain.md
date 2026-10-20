---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Full image chain: harness layer, overrides, Refresh, Prune

**Why:** phase 1 runs one image for every project ([Image builder](./archive/image-builder.md)). The target ([Agent images](../product/architecture/tech-stack.md#agent-images)) separates the layers so harnesses update without rebuilding toolchains, and so the CEO and each project can add tools.

**Done when:** the four-layer chain (base → harness with every supported harness → global → project) is built, with global and project Containerfile overrides as transparent tags when absent, cascade invalidation, Refresh with optional prune, Prune, and the full [Settings → Maintenance](../product/features/settings.md#maintenance) screen.

- [ ] split the phase 1 Containerfile into base and harness layers; add Pi (and Hermes if it fits)
- [ ] global and project overrides (where the global one lives: [Tech stack → Open questions](../product/architecture/tech-stack.md#open-questions)), `BASE` build argument, transparent tags
- [ ] Refresh (rebuild without cache, then the layers below) with optional prune; Prune by label
- [ ] the full Maintenance screen (chain rows, where a Containerfile would go)
- [ ] each image's build time and the harness versions inside it (Maintenance rows, and `found · <version>` on Settings → Harnesses): phase 1 shows neither
