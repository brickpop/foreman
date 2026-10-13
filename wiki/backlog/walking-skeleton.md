---
type: task
status: in-progress
priority: high
tags: [feature]
---

# Walking skeleton: one agent in Podman, live in the browser, calling a team tool

The first code after the design phase: the thinnest end-to-end slice through every risky part of the design. It **confirms or corrects the decisions taken on paper**; whatever it finds is written back into the specs it links, in the same change. Everything else is built on top of it.

The slice: `foreman` starts in a workspace with one project, builds the image chain, starts one Claude Code instance in a rootless Podman container, shows its TUI live in the browser, and the agent calls one team tool that Foreman receives.

Repo and tooling:
- [ ] Repo layout, justfile (`dev`, `build`, `test`, `e2e`, `lint`), README setup steps, as in [Tech stack](../product/architecture/tech-stack.md#repo-layout)
- [ ] Access token: URL printed and opened at start, cookie, Host/Origin checks ([Security](../product/architecture/security.md#access-token))

Runtime facts to verify, on Linux (Fedora, SELinux) and macOS (`podman machine`):
- [ ] Podman driven from Go through the libpod REST API: create a TTY container, attach, resize, stop, list by label, re-attach after Foreman restarts ([Session runner](../product/architecture/session-runner.md#podman-runner-production))
- [ ] Image chain built through the API: base → harness → transparent workspace and project layers; Refresh; a scaffolded override ([Agent images](../product/architecture/tech-stack.md#agent-images))
- [ ] Credentials: `claude /login` in the login container; the shared login file used read-write by 12 concurrent instances across several refresh cycles; how to point each harness at it (file mount, symlink or folder). Fallbacks if it fails: keeper, or per-instance logins. Same for Pi ([Credentials](../product/architecture/session-runner.md#credentials))
- [ ] Same-path mounts, `keep-id`, `:z` on SELinux; bind-mount speed on macOS
- [ ] System prompt injection and MCP registration for Claude Code, then Codex ([Harness adapters](../product/architecture/harness-adapters.md#open-questions))
- [ ] The container reaches Foreman's team-tools MCP server at `host.containers.internal` with a per-instance token, while the UI listener stays on localhost ([Team tools](../product/architecture/team-tools.md#open-questions))
- [ ] How long a harness lets an MCP tool call block (for `escalate(approval)`)
- [ ] Idle and waiting detection through harness hooks (Claude Code hooks, Codex `notify`)
- [ ] Typing into a busy TUI: queued input and a reliable Enter
- [ ] xterm.js over the binary WebSocket: latency under 50ms, resize, scrollback replay from the log

Done when the slice runs on both platforms and every item above is written back into its spec (decided, or reopened as a question for the CEO). The specs' remaining open questions are exactly these facts: [Harness adapters](../product/architecture/harness-adapters.md#open-questions) and [Team tools](../product/architecture/team-tools.md#open-questions).

## Steps

Each step ends with tests passing here and a commit prepared for the CEO. Podman steps come with `just` recipes the CEO runs on Fedora and macOS.

1. [ ] Server shell: Go module, `foreman [dir]` with the first-run prompt, access token (URL, cookie, Host/Origin checks), embedded SPA placeholder, justfile, README
2. [ ] Terminal path, hermetic: Runner interface, local PTY runner, fake harness, `/ws/pty/:instance`, xterm.js page, session log replay
3. [ ] Podman runner over the libpod REST API, and the image chain (build, tag, refresh, prune), with check recipes
4. [ ] Claude Code in a container: login container, shared login file, session prompt file, MCP config
5. [ ] Team-tools listener with one tool, reachability from containers, harness hooks for idle detection
