---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Reach a server an agent runs in its container

**Why (the CEO, 2026-10-17, very low priority):** an agent may start a dev server (say on port 8080) inside its container, and the CEO wants to open it from their browser. Today nothing in a container is reachable from outside: the only network path Foreman sets up goes the other way, from `foreman-agent` to the agent port on the host's loopback ([Building blocks](../product/architecture/building-blocks.md)). Only the Podman engine is concerned; the direct engine runs agents as host processes, where a server is already reachable. An agent opening ports itself (some port-forwarding tool) is out of scope for now.

**To decide when the card starts:**
- **When ports are opened.** Podman publishes ports when a container is created, not on a running one. So either a fixed set per agent at start (from the project or team config), or a proxy Foreman runs on the host that forwards to the container's address on demand, which works on a running container and fits "open what the agent just started" better.
- **Where they listen.** On the host's loopback only (`127.0.0.1`), never `0.0.0.0`, unless the CEO switches Foreman to the local network, like the UI.
- **Which host port.** The container's own port (8080) collides as soon as two agents run the same dev server. Foreman picking a free one per agent and showing it is the likely answer.
- **How the CEO learns of it.** A row in the agent's Sessions pane ("tove: 8080 → localhost:51234, Open"), from the ports the container listens on, or from what the agent reports.
- **Security.** A dev server is agent-written code, so opening it in the CEO's browser runs that code on Foreman's machine's loopback. It must not share an origin with Foreman's UI (another port is another origin, so the access cookie isn't sent).

**Done when:** the CEO opens a server an agent started in its container from a link in Foreman, listening on loopback only, with no clash between agents; tests cover the port choice and the bind address.

- [ ] decide the points above with the CEO
- [ ] spec (Session runner, Sessions), the engine side, the UI, tests
