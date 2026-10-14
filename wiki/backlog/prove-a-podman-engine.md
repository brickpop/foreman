---
type: task
status: todo
priority: high
tags: [spike]
---

# Experiment A: drive Podman from Go through its REST API

Epic: [Prove](./epics/prove.md)

**Question:** can Foreman's Podman engine start a container with a terminal, stream it both ways, resize it, stop it, and find and re-attach to it after Foreman restarts, using Podman's REST API from plain Go (no podman CLI, no bindings module)? See [Building blocks → What is proven](../product/architecture/building-blocks.md#what-is-proven-and-what-isnt), [Session runner](../product/architecture/session-runner.md).

Claude writes the recipe; the CEO runs it on macOS and Fedora.

**Procedure** (`just prove-a`, a small Go program under `cmd/prove/`, removed when the epic ends):
1. Find the Podman API socket; on macOS, start the default machine if it isn't running.
2. Create and start a container from a small public image (`docker.io/library/alpine`) running `/bin/sh` with a TTY, labelled `foreman.prove=a`, the current folder bind-mounted at the same path (with `:z` on SELinux).
3. Attach; write `stty size; echo hello` + Enter; expect `hello` and the initial size back.
4. Resize to 100×30 through the API; write `stty size`; expect `30 100`.
5. Exit the Go program without stopping the container; start it again; list containers by label; re-attach; write `echo again`; expect `again`.
6. Stop with a grace period; expect the exit code; remove the container.
7. Time each step (start latency, round-trip latency of a keystroke).

**PASS:** every step works on both machines, and a keystroke's round trip stays under 50 ms locally. **FAIL:** any step doesn't, in which case the output says which and why.

**On FAIL:** the engine switches to the fallback in [Session runner](../product/architecture/session-runner.md) (the `podman` CLI driven through a local pseudo-terminal for attach), and the spec records why.
