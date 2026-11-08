---
type: epic
status: todo
order: 5
---

# Director

**Goal:** one agent above the teams. The CEO talks to it once, and it dispatches the work to the leads of several projects, follows it, and reports back; it also creates, hires into and refines teams (what the Chief of Staff does today), and proposes deployments. It replaces the [Chief of Staff](../../product/features/chief-of-staff.md), renamed.

**Usable when done:** the CEO asks the Director for an outcome spanning several projects; it asks each lead, the leads report back as they go, the Director's board shows where each project stands, and the CEO still decides every request and merge.

## Decided (the CEO, 2026-11-05 to 2026-11-08)

The design conversation, consolidated. Each card below carries the detail of its part; this list is the whole picture.

- **One agent, renamed.** The Chief of Staff and the cross-project coordinator are one agent, the **Director**, everywhere (UI, specs, code, ids, routes). It reads every project's HQ, talks to leads, manages teams. **New team** opens the Director with "Help me create a new team."
- **It runs standing, not only while its page is open**, so leads' reports reach it while the CEO is away. It sleeps and wakes like any agent ([Sleep after a quiet spell](../sleep-when-quiet.md)).
- **Dispatch and report only.** It asks **leads** (never members) to do things and collects their reports. It never answers a lead's request: approvals, questions, reviews and merges stay the CEO's. The CEO keeps talking to any lead or agent directly at any time; on a conflict, **the CEO wins**, and the leads' prompt says so.
- **Leads report only on work the Director started.** A Director ask can be replied to several times (progress), the last reply marked `done`. A lead has no way to start a conversation with the Director: work the CEO starts in a lead's session has no thread to it. Enforced by Foreman, not only the prompt.
- **It proposes; Foreman writes, on the CEO's click.** Team files (as today) and deployments (deploy, stop). Its mounts make this hold: its HQ is read-only except its board.
- **Its board:** one `outcome` card per outcome the CEO asked for, with the projects involved, each one's state, the order, and a log. It lives in **Foreman's HQ**, `~/foreman/hq/`.
- **One team per project stays** (the CEO, 2026-11-05). Parallel work goes inside the team (the lead runs several streams), or, for two truly separate efforts on the same code, two projects over the same repos, each with its team, coordinated by the Director.
- **The folders** ([One Foreman folder](../foreman-folder.md)): `~/foreman/` (a setting, `foreman_folder`) holds what the CEO owns, `~/.foreman/` Foreman's machinery. Foreman's HQ (`~/foreman/hq/`: the Director's board, `teams/`, `instructions.md`) and each project's HQ (`~/foreman/projects/<p>/hq/`) are the same kind of thing. The global repo goes.
- **Security, accepted:** the Director is the one path between projects (a lead's reply in one project becomes the Director's input, which instructs another project's lead). Mitigated: no code, no git, no secrets; read-only on every HQ; replies framed as quoted data and capped; everything it sends shows in the project's Activity. Written into [Security](../../product/architecture/security.md) by [Director and leads](../director-leads.md).
- **Dismissed for now:** harness timers (Claude Code's `ScheduleWakeup`, `/loop`, session cron) are lost when an agent sleeps; not worked around.

## Cards, in order

1. [Sleep after a quiet spell](../sleep-when-quiet.md) (independent: can go first)
2. [Chief of Staff → Director](../director-rename.md)
3. [One Foreman folder](../foreman-folder.md), then [The wiki at the HQ's root](../wiki-at-root.md) (in [Install and first run](./install-first-run.md))
4. [The Director as a standing agent](../director-standing.md)
5. [The Director and the leads](../director-leads.md)
6. [The Director's board](../director-board.md)
7. [The Director proposes deployments](../director-deploys.md)

Already in this epic: [Create a team, hire, refine](../cos-create-team.md) (in progress), [The lead proposes changes to its own team](../lead-proposes-team.md), [A Teams page](../teams-page.md), [Project config with the Chief of Staff](../cos-project-config.md) and the decision on [Starter teams](../starter-teams.md).

**Journey steps:** 7 and 25 of [the journey](../../product/journey.md); 27 (project config) later; the cross-project steps get written by [The Director and the leads](../director-leads.md).

**Exit criteria:** the steps above work end to end on the real server, with tests on the fake harness.
