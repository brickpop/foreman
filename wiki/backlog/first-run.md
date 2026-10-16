---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# First run: sensible defaults and a setup checklist

**Why:** today a new workspace works only if the CEO already knows what agents need, and each missing piece shows up as a refusal: Foreman doesn't start without Podman, the agent image builds for minutes on the first deploy, a harness login is found through the Deploy dialog's Connect row, and the engine, `--fake-harness` and `just dev`'s workspaces can contradict each other (the CEO hit that on 2026-10-16). Replaces the "First-run onboarding" idea of 2026-10-15.

**Proposed defaults** (to confirm with the CEO before building):
- **Foreman always starts**, whatever is missing, and says what is missing in the UI rather than at the terminal. Only a busy port or an unreadable workspace stops it.
- **The engine is Podman**, never switched on its own. `direct` stays an explicit setting for development, with its warning. Foreman notices Podman once it is installed or its machine is running, with no restart.
- **On macOS, Foreman offers to create and start the Podman machine** (one button, with Podman's own output shown), since that is the step people miss.
- **The agent image builds in the background as soon as Podman is ready**, not on the first deploy; its progress shows in the checklist and in Settings → Maintenance.
- **A setup checklist on Home** while anything is missing, in order: Podman (installed, and running), the agent image, each harness the workspace's teams use (Connect), a project, a team. Each row says what to do and has its button; it goes away when everything is done, and comes back if something breaks (Podman stopped, logged out).
- **Deploy refuses with the first missing step**, linking to it (it already does so for a harness login).
- **Development stays out of the way:** `--fake-harness` and `--ui-dir` are development flags only, and `just dev` (Claude Code) and `just dev fake` each have their own workspace (done 2026-10-16).

**Done when:** a new workspace on a machine with nothing set up reaches a first deploy by following the checklist alone; each missing piece shows as a checklist row, never as a startup error.

- [ ] the CEO confirms the defaults above
- [ ] Foreman starts without Podman; the engine connects when Podman becomes available (no restart); the instance manager and Maintenance say "Podman isn't running" meanwhile
- [ ] macOS: create and start the Podman machine from the UI
- [ ] the image builds in the background once Podman is ready
- [ ] `GET /api/setup`: each step and its state, with `setup.changed` events; the Home checklist
- [ ] Deploy refuses with the first missing step
- [ ] Linux: verify the containers' host alias reaches the agent port (the open item from [experiment C](./prove-c-agent-port.md)), so the checklist is right there too. Nothing ships or runs beside Foreman either way: a fix is a network option in the container spec Foreman sends Podman (pasta mapping the host loopback), or the agent port bound to the host side of Podman's network (never 0.0.0.0)
- [ ] tests: each step's state, Podman appearing after startup, the image build starting on its own
