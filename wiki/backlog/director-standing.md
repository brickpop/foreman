---
type: task
status: blocked
priority: high
tags: [feature]
epic: director
---

# The Director as a standing agent

**Why:** the CEO, 2026-11-05. Today the Chief of Staff is an ad-hoc session that stops a second after the CEO leaves its page ([Chief of Staff → How it runs](../product/features/chief-of-staff.md#how-it-runs)). The [Director](./epics/director.md) waits for leads' reports for hours, so it has to live like an agent: restarted after a crash, put to sleep when quiet, woken by a message, continuing its conversation.

Blocked by [Chief of Staff → Director](./director-rename.md), [One Foreman folder](./foreman-folder.md) and [The wiki at the HQ's root](./archive/wiki-at-root.md) (its mounts and its prompt name those paths), and [Sleep after a quiet spell](./sleep-when-quiet.md).

**Decided:**

- **An instance with no deployment**, id `director`: the instance manager runs it like a member (starts, statuses from hooks, crash restarts, restore after a Foreman restart, the sleep and the wake). Today every instance has a deployment (`store.Instance.DeploymentID`) and every tool caller carries one (`teamtools.Caller`): this card makes the deployment optional in one place, rather than a second runtime beside the manager.
- **It holds a session slot** while running and counts towards `max_sessions` (the Chief of Staff didn't). It sleeps after `sleep_after_minutes` quiet like the others.
- **Its harness:** the `default` profile's, as now. Its conversation lives in `~/.foreman/harness/director/`.
- **Leaving its page no longer stops it.** The page shows its terminal like any agent's session: asleep, it shows the asleep view with Start. An entry point with an opening line (`?start=create-team`, hire, refine) wakes it if needed and types the line.
- **Mounts** (read-only unless said):
  - Foreman's HQ, `~/foreman/hq/`, as its working folder, **read-only**, with its board folder `hq/outcomes/` layered on top **read-write**. So `teams/` and `instructions.md` can't be written from inside: team changes go through `propose_team` and the CEO's click, as today. (A folder, not a file, is layered: an editor that saves by rename would break a single-file mount.)
  - every project's HQ, read-only, to read goals, boards and specs. Never a code repo, a worktree, `files/`, the home's secrets, `state.db`, logs or settings.
  - the profile's login file and `foreman-agent`, as for any agent.
- **Its board's commits:** agents never write git; the Director commits its board through `commit`, which for it takes only files under `hq/outcomes/` (anything else refused), authored as the Director.
- **Its prompt**, composed at each start: who it is and what it may do (dispatch and report; it never answers for the CEO), Foreman's HQ layout, the team format (as today), the projects with their team and lead, its tools.

**Open, to settle while building:** where its own blocked requests show (a crash loop, a stuck turn): requests belong to a project today. Proposal: on the Director's page and the Dashboard.

**Steps:**

- [ ] the instance manager: an instance without a deployment; the Director's lifecycle on it (crash, restore, sleep, wake)
- [ ] mounts, including the read-write board over the read-only HQ, on Podman (Linux and macOS) and Direct
- [ ] `commit` for the Director, limited to `hq/outcomes/`
- [ ] the page: no stop on leaving, the asleep view, entry points waking it
- [ ] specs: the Director's (How it runs), [Agent runtime](../product/architecture/agent-runtime.md), [Session runner](../product/architecture/session-runner.md), [Building blocks](../product/architecture/building-blocks.md) (it becomes an instance), [Security](../product/architecture/security.md) (its mounts)
- [ ] tests: survives leaving the page and a Foreman restart; sleeps and wakes with its conversation; can't write `teams/`; `commit` refuses outside `outcomes/`; counts towards the limit
