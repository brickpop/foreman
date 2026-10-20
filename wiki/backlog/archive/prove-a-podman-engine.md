---
type: task
status: done
priority: high
tags: [spike]
epic: prove
---

# Experiment A: drive Podman from Go through its REST API

**Question:** can Foreman's Podman engine start a container with a terminal, stream it both ways, resize it, stop it, and find and re-attach to it after Foreman restarts, using Podman's REST API from plain Go (no podman CLI, no bindings module)? See [Building blocks → What is proven](../../product/architecture/building-blocks.md#what-is-proven-and-what-isnt), [Session runner](../../product/architecture/session-runner.md).

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

**On FAIL:** the engine switches to the fallback in [Session runner](../../product/architecture/session-runner.md) (the `podman` CLI driven through a local pseudo-terminal for attach), and the spec records why.

## Result on macOS (2026-10-14): PASS

Podman 5.7.1 on a `podman machine` (Apple Silicon), driven only through the REST API from Go (`just prove-a`):
- API socket found through `podman machine inspect` (`…/podman-machine-default-api.sock`).
- Create + start with a terminal: 84 ms. Pulling alpine: 2.6 s (first time).
- Two-way terminal stream over the attach endpoint; the initial size and an API resize are seen by the program (`stty size`).
- The bind-mounted folder is the working directory at the same path as on the host.
- `detachKeys=` (empty) disables Podman's detach sequence: Ctrl-P passes through and the stream stays attached. (Ctrl-Q needed `stty -ixon`: in its default mode a terminal consumes Ctrl-S/Ctrl-Q as flow control, before any program; full-screen programs switch that off.)
- Keystroke round trip: median 2.2 ms, worst 2.2 ms (target: under 50 ms).
- A second process finds the container by label, re-attaches, works both ways, stops it with a grace period and reads the exit code.
- Seen on the way: the alpine shell queries the cursor position (`ESC[6n`) at every prompt, so programs do ask the terminal questions even with nobody watching ([Terminal state](../../product/architecture/terminal-state.md)).

**Linux:** not yet run (no Podman there yet). The CEO accepted macOS as enough for now (2026-10-14); re-run on Linux when a machine with Podman is available.
