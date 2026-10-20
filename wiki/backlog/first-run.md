---
type: task
status: in-progress
priority: high
tags: [feature]
epic: install-first-run
---

# First run: sensible defaults and a setup checklist

**Why:** today a new workspace works only if the CEO already knows what agents need, and each missing piece shows up as a refusal: Foreman doesn't start without Podman, the agent image builds for minutes on the first deploy, a harness login is found through the Deploy dialog's Connect row, and the engine, `--fake-harness` and `just dev`'s workspaces can contradict each other (the CEO hit that on 2026-10-16). Replaces the "First-run onboarding" idea of 2026-10-15.

**Defaults** (confirmed by the CEO, 2026-10-16):
- **Foreman always starts**, whatever is missing, and says what is missing in the UI rather than at the terminal. Only a busy port or an unreadable home stops it.
- **The engine is Podman**, never switched on its own. `direct` stays an explicit setting for development, with its warning. Foreman notices Podman once it is installed or its machine is running, with no restart.
- **On macOS, Foreman offers to create and start the Podman machine** (one button, with Podman's own output shown), since that is the step people miss.
- **The agent image builds in the background as soon as Podman is ready**, not on the first deploy; its progress shows in the checklist and in Settings → Maintenance.
- **A setup checklist on Home** while anything is missing, in order: Podman (installed, and running), the agent image, each harness the teams use (Connect), a project, a team. A project without a code repo is a step too (its link, Add a repo, opens the project's Settings, where Code repos adds one). Each row says what to do and has its button; it goes away when everything is done, and comes back if something breaks (Podman stopped, logged out).
- **Deploy refuses with the first missing step**, linking to it (it already does so for a harness login).
- **Development stays out of the way:** `--fake-harness` and `--ui-dir` are development flags only, and `just dev` (Claude Code) and `just dev fake` each have their own home (done 2026-10-16).

**The machine's size** (the CEO, 2026-10-16): the Create button opens a small form. Memory starts at 4 GB and goes up in 1 GB steps to 60% of the Mac's, prefilled with half the Mac (kept within those limits). CPUs go from 1 to all of the Mac's cores, prefilled with half. A Mac with too little memory for 4 GB within 60% gets the reason rather than the form. Podman's own defaults (often about 2 GB) fit only 2–3 agents.

**Design** (built; the spec is [First run](../product/features/first-run.md)):
- **The Podman engine connects lazily.** `podman.Open` returns the engine at once. `Connect` finds the socket and checks it answers, starting the machine (macOS) or the service (Linux) as today; it does nothing if the current connection answers. Images reach Podman through the engine, so they follow a reconnection.
- **A watcher** in the composition root tries `Connect` every few seconds while Podman isn't answering, and on each request for the setup's state. The first time it connects, it restores the instances (`Restore`), delivers what waited (`Sweep`), and starts the image build in the background. Each change of state publishes `setup.changed`.
- **Until then:** deploying and waking an agent refuse with "Podman isn't running" (409). Instances from before are restored only once Podman answers.
- **The setup block** (`internal/setup`) computes the steps in order: Podman installed, Podman running (on macOS: the machine exists, then runs), the agent image, each harness the teams use, a team, a project, each project's code repo. Each step is done, to do, working or failed, with a detail. The Direct engine has no Podman or image steps.
- **Machine creation:** `POST /api/setup/podman-machine {cpus, memory}` runs `podman machine init --now` with that size; its output is kept like a build log and shown in the row.
- **Deploy refuses** with the first missing step that blocks it. While the image is building, deploying is allowed and the lead waits in `starting`, as today.

**Done when:** a new install on a machine with nothing set up reaches a first deploy by following the checklist alone; each missing piece shows as a checklist row, never as a startup error.

- [x] the CEO confirms the defaults above (2026-10-16: all four as proposed)
- [x] Foreman starts without Podman; the engine connects when Podman becomes available (no restart); deploying and waking refuse with `NOT_READY` meanwhile, and Maintenance says Podman isn't reachable
- [x] macOS: create and start the Podman machine from the UI (sized by the CEO), and Start a stopped one
- [x] the image builds in the background once Podman is ready
- [x] `GET /api/setup`: each step and its state, with `setup.changed` events; the Home checklist
- [x] Deploy refuses with the first missing step (the server with `NOT_READY` before its other checks; the dialog beforehand)
- [ ] Linux: verify the containers' host alias reaches the agent port (the open item from [experiment C](./archive/prove-c-agent-port.md)), so the checklist is right there too. Nothing ships or runs beside Foreman either way: a fix is a network option in the container spec Foreman sends Podman (pasta mapping the host loopback), or the agent port bound to the host side of Podman's network (never 0.0.0.0)
- [x] tests: each step's state, Podman appearing after startup and coming back, only launch and Start starting the machine, the restore retried, the image build starting on its own, deploy and wake refused meanwhile, the routes, the machine's bounds
- [x] on the Mac: `just test-podman` (Open, Connect, State), the checklist seen live: stop the machine (Start), and on a Mac without a machine the form (or with `podman machine rm` on a spare machine)
