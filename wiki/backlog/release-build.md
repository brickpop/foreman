---
type: task
status: in-progress
priority: high
tags: [feature]
epic: install-first-run
---

# Release build: everything embedded, a foreman-agent per CPU type

**Why:** a release `foreman` must run with nothing else beside it. Today `just build` embeds the UI, the agent image's Containerfile and foreman-agent, but foreman-agent only for the build machine's CPU type, and a plain `go build ./cmd/foreman` gives a binary with no foreman-agent at all: it starts, prints one line, and agents silently get no team tools ([Team tools → Transport](../product/architecture/team-tools.md#transport)).

**Why embedded, still:** foreman-agent is mounted at start, not baked into the image (decided 2026-10-15), so it always matches the Foreman that runs it. The alternatives are worse: downloading it needs the network and a place to host it; building it on the machine needs Go. Two Linux builds cost about 16 MB (8 MB each, stripped); compressing them is possible if the size ever matters.

**Done when:** `just build` embeds foreman-agent for `linux/amd64` and `linux/arm64`; Foreman mounts the one for its own CPU type; a build without them refuses to build instead of shipping a Foreman without team tools; old copies in the cache folder are removed.

**Decided on the way (2026-10-16):**
- **Foreman's own CPU type (`runtime.GOARCH`), not the engine's:** the CEO's call, so nothing in the build or the choice depends on Podman. A container runs on Foreman's CPU type: on Linux it is the host's, on macOS the Podman machine runs on the Mac's. The build machine needs no Podman either (Go cross-compiles both).
- **A `release` build tag** rather than a test: `just build` compiles with `-tags release`, whose embed lists both foreman-agents and the UI by name, so the compiler refuses a build missing any. Without the tag (`just dev`, tests, a fresh checkout) whatever is built is embedded, and Foreman says at startup when the team tools are missing.
- **Old copies:** each Foreman refreshes its copy's time when it starts; copies no Foreman has started with for 7 days are removed.

**Edge cases:**
- **An amd64 Foreman under Rosetta on an Apple silicon Mac** (someone downloading the wrong binary) mounts the amd64 foreman-agent into arm64 containers, and the team tools fail. It only works if the Podman machine has Rosetta enabled. Published binaries (Not yet scheduled) should make the right one obvious, or ship a universal macOS binary.
- **A Foreman left running for more than 7 days** across an upgrade: the new version's startup removes the old one's copy, and the old Foreman's next container start fails to mount it. Restarting the old Foreman fixes it.
- **The Direct engine never runs foreman-agent** (the fake harness speaks the link itself), so on a Mac it is a Linux file that is mounted nowhere. Unchanged.
- **`just lint` doesn't vet the `release` files**: vetting them needs the built files. `just build` compiles and tests them.

- [x] `just agent` builds both CPU types into `agentbin/dist/` (`foreman-agent-amd64`, `foreman-agent-arm64`), with Go alone; `agentbin.Binary(arch)`
- [x] Foreman writes the one for `runtime.GOARCH` (`cmd/foreman/agent.go`)
- [x] `-tags release`: the build fails without both foreman-agents and the UI (checked: removing one stops the compile); `go test -tags release ./agentbin ./ui` in `just build` checks each foreman-agent is a static Linux program for its CPU type, and that every file `index.html` loads is embedded
- [x] old copies: refreshed at start, removed after 7 unused days (`TestWriteAgent`)
- [x] the release binary, run from an empty folder, serves the UI and writes its foreman-agent (checked by hand on Linux arm64); the Containerfile was already embedded, and `just test-podman` builds images from it
- [x] the README's install section: one binary, plus Podman
- [ ] on the Mac: `just build`, and `just test-podman` (its new test: a container runs on Foreman's CPU type)
