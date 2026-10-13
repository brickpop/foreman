---
type: task
status: todo
priority: medium
tags: [spike]
---

# Spike: drive Podman from Go via the libpod API

With plain `net/http` over the Podman socket: create a TTY container, attach (bidirectional, hijacked stream), resize, stop, list by label, and re-attach after the Go process restarts. Compare with shelling out to `podman`. Feeds [Session runner](../product/architecture/session-runner.md#open-questions).

- [ ] Attach, resize, re-attach after restart
- [ ] Build an image through the API (the [layered images](../product/architecture/tech-stack.md#agent-images))
- [ ] Write the findings into the specs, then delete this card
