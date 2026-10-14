---
type: task
status: done
priority: high
tags: [spike]
epic: prove
---

# Experiment C: a container reaching Foreman's agent port on the host's loopback

**Question:** can a process inside a rootless Podman container reach a port that Foreman opens **only on the host's loopback** (127.0.0.1), through `host.containers.internal`, on macOS (`podman machine`) and on Fedora (pasta networking)? And is that port unreachable from the local network? See [Team tools → Transport](../product/architecture/team-tools.md#transport), [Building blocks → Agent link](../product/architecture/building-blocks.md#4-agent-link-agentlink--foreman-agent).

Claude writes the recipe; the CEO runs it on macOS and Fedora.

**Procedure** (`just prove-c`):
1. Start a tiny HTTP server on the host at `127.0.0.1:<port>` that answers with the caller's token.
2. Start a container (alpine) and `wget` `http://host.containers.internal:<port>/` with a bearer token.
3. From the host, check that the port is not reachable on the machine's LAN address.
4. Repeat with 12 containers at once; measure the round trip of a small request.

**PASS:** containers reach the loopback port on both machines, the LAN can't, and the round trip is in the low milliseconds. **FAIL:** the alias doesn't reach the host's loopback on one of them.

**On FAIL:** the agent port binds to the address Podman's network does reach (the host side of Podman's network, never 0.0.0.0), or Podman's own port mapping is used for that platform; [Team tools → Transport](../product/architecture/team-tools.md#transport) and [Security](../product/architecture/security.md) record the change.

## Result on macOS (2026-10-14): PASS

Podman 5.7.1 on a `podman machine` (`just prove-c`):
- A port bound only to the host's `127.0.0.1` is reachable from containers at `host.containers.internal`.
- Each request is attributed to its container by its bearer token; a wrong token is refused.
- The port is not reachable on the Mac's LAN address.
- 12 containers at once, 10 requests each: all arrived and were attributed correctly.

**Linux:** not yet run (no Podman there yet; on Linux the path is pasta networking, which may behave differently). The CEO accepted macOS as enough for now (2026-10-14); re-run on Linux when a machine with Podman is available.
