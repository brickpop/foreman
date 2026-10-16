---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# Release build: everything embedded, a foreman-agent per CPU type

**Why:** a release `foreman` must run with nothing else beside it. Today `just build` embeds the UI, the agent image's Containerfile and foreman-agent, but foreman-agent only for the build machine's CPU type, and a plain `go build ./cmd/foreman` gives a binary with no foreman-agent at all: it starts, prints one line, and agents silently get no team tools ([Team tools → Transport](../product/architecture/team-tools.md#transport)).

**Why embedded, still:** foreman-agent is mounted at start, not baked into the image (decided 2026-10-15), so it always matches the Foreman that runs it. The alternatives are worse: downloading it needs the network and a place to host it; building it on the machine needs Go. Two Linux builds cost about 16 MB (8 MB each, stripped); compressing them is possible if the size ever matters.

**Done when:** `just build` (and the release build) embeds foreman-agent for `linux/amd64` and `linux/arm64`; Foreman mounts the one that matches the engine's CPU type (Podman reports it: on a Mac it is the machine's, not the Mac's); a build without them refuses to build instead of shipping a Foreman without team tools; old copies in the cache folder are removed.

- [ ] `just agent` builds both CPU types into `agentbin/dist/` (`foreman-agent-amd64`, `foreman-agent-arm64`); `agentbin.Binary(arch)`
- [ ] the engine says its CPU type (Podman's `info`; the Direct engine: the host's); `writeAgent` writes that one
- [ ] no foreman-agent embedded: the build fails (a test in `agentbin` that `just build` and CI run), not a startup message; `just dev` keeps building them
- [ ] at startup, `foreman-agent-*` files in the cache folder other than the current ones are deleted (each version leaves a copy today, a few MB each); not while another Foreman uses them (an older Foreman still running on another workspace): delete only files older than a day, or keep them per version
- [ ] the image's Containerfile and the UI were already embedded: a test that the release binary serves the UI and builds the image with no file beside it
- [ ] the README's install section: one binary, plus Podman
