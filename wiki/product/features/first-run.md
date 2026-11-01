---
type: spec
status: built
tags: [feature, ui, setup]
---

# First run

A new home works out of the box: `foreman`, with no folder and no question, makes `~/.foreman` ([Home](../models/home.md#starting-foreman)). Foreman starts whatever is missing, says what's left on [Dashboard](./dashboard.md), and reaches Podman as soon as it's there. There's no flag, setting or command to know beforehand, and nothing to discover by hitting a refusal. The defaults were confirmed by the CEO on 2026-10-16 ([First run card](../../backlog/first-run.md)).

## Foreman always starts

- Only a busy port, an unreadable home or a folder argument stops `foreman`; a second `foreman` prints the first one's URL instead. Podman missing, stopped or failing is a checklist row, not a startup error.
- **The engine is Podman**, and is never switched on its own. `direct` stays an explicit development setting ([Global settings](../models/global-settings.md)).
- **The watcher** (the setup block, `internal/setup`) reaches Podman at launch, then checks every 3 seconds, and at once after the CEO's Start or a machine creation ([The Podman service](../architecture/session-runner.md#the-podman-service)).
  - At launch it starts a stopped macOS machine, as Foreman always has. Later checks never do, so a machine the CEO stopped stays stopped until they press **Start**.
  - On Linux it starts the rootless service whenever nothing answers.
- **The first time Podman answers**, Foreman restores the instances of active deployments (re-attaching their containers, [Agent runtime](../architecture/agent-runtime.md#restarts)); once that is done and agents may start, it delivers the messages that waited, waking asleep recipients. It then builds the agent image in the background, so the first deploy doesn't wait minutes for it.
- **Each later return** of Podman delivers what waited meanwhile and checks the image again.
- **Until then, deploying and waking an agent refuse** with `409 NOT_READY` ("agents can't start yet: podman isn't running: see the setup checklist on the Dashboard"). Messages for a sleeping agent wait.
- **The Direct engine** needs none of this: it is ready at once, with no Podman or image steps.

## The checklist

Dashboard shows it above the requests while any step isn't done, and hides it once all are. It comes back if something breaks (Podman stopped, a login gone). `GET /api/setup` returns the steps in order ([API](../architecture/api.md#endpoints)). The UI refetches on `setup.changed`, `config.changed` and `projects.changed`, and polls every 3 s while a step is left (every second while one is working), since installing Podman or logging in happens outside Foreman.

Header: `Setup · N of M done`, then one panel, one row per step. Each row has a state mark (done: a green check; to do: an empty ring; working: a spinning ring; failed: red), the title, and the detail when not done. A row that is working or failed shows the last lines of its output in a small terminal box.

| Step | Shown | To do | Action |
|---|---|---|---|
| **Podman installed** | Podman engine | `Install Podman: brew install podman` (macOS) or `sudo dnf install podman (or your distribution's package)` | none: installed outside Foreman, seen within seconds |
| **Podman machine** | Podman engine, macOS | `Create the Podman machine, where agents run` | the machine form (below); working while it is created, with Podman's output |
| **Podman running** | Podman engine | waits for the steps above; `The Podman machine is stopped (Start)`; a connection failure shows the error | **Start** when the machine is stopped. While the instances are restored it is working (`Restoring the agents`). |
| **Agent image** | Podman engine | `Builds once Podman runs`; working: `Building the agent image (a few minutes, once)` with the build output | failed: a link to Settings → [Maintenance](./settings.md#maintenance) (Rebuild) |
| **The `<profile>` profile connected** | each profile the projects use (`default` while there is no project) | `Not connected yet: its agents can't start`; for one that doesn't exist, the step fails: `No such profile: create it under Settings → Agents → Profiles, or pick another in the project's settings` | **Connect**: the login terminal ([Settings → Profiles](./settings.md#profiles)); for one that doesn't exist, a link to Profiles |
| **A team** | always | `Create a team` | **Create a team**: the [Chief of Staff](./chief-of-staff.md)'s create-team flow |
| **A project** | always | `Add a project` | **`New project`** opens the [New project](./add-project.md#the-dialog) dialog |
| **A code repo in `<project>`** | each project with no code repo (and no problems) | `<project> has no code repo for agents to work in: add one` | **`Add a repo`**, a link to the project's [Settings](./project-settings.md) section, where Code repos adds one |

## The Podman machine (macOS)

Podman's own defaults (often about 2 GB of memory) fit only 2–3 agents, so the **Create machine** row asks for a size (the CEO, 2026-10-16):

- **Memory:** from 4 GB in 1 GB steps, up to 60% of the Mac's memory, prefilled with half the Mac (kept within those limits).
- **CPUs:** from 1 to all of the Mac's cores, prefilled with half.
- A Mac with too little memory for 4 GB within 60% gets the reason instead of the form.
- A note under the form: it downloads Podman's machine image (about 1 GB) once, each agent needs a few hundred MB, and `podman machine set` resizes it later.

**Create machine** runs `podman machine init --now --cpus N --memory M` in the background (`POST /api/setup/podman/machine`). The row turns to working, with Podman's output, and the next check connects to the running machine. A size out of bounds, or a machine that already exists, is refused (`409 CANT_CREATE`), and the message shows under the form.

## Deploying

The [Deploy dialog](./deploy-team.md) shows `Podman isn't running yet, so agents can't start.` with a link to the setup on the Dashboard, and keeps Deploy disabled while Podman isn't running. This works like its row for a harness without a login. While the agent image is still building, deploying is allowed, and the lead waits in `starting` until it's ready.

## Edge cases

- **Podman stops while agents run** (a stopped machine): their containers end, the agents crash, and their restart fails, so they end up `stopped`. Once Podman is back, each needs **Start again**, but messages for them still wait. (Not yet handled: queueing their restart until Podman returns.)
- **Instances shown before the restore:** until Podman first answers, the instances of active deployments keep the statuses saved when Foreman last ran, though nothing is attached yet.
- **A deploy right after Podman stopped:** the watcher notices within 3 seconds. A deploy in that window passes the gate, and its lead's start fails (`stopped`).
- **A second Foreman on the same Mac** sees the same machine. Creating one is refused when it exists.
