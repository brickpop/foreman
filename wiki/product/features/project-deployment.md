---
type: spec
status: built
tags: [feature, ui, git, pipeline]
---

# Project deployment

A project's **Deployment** section ([Project page](./project-page.md)): the team on the project, its agents with what each is doing and the branch it has checked out, then the work on its way to `main` and what merged. It replaced the Worktrees tab and Project settings' "Teams on this project" on 2026-10-31, both of which listed the same agents (the CEO: remove redundant info). A project has one team at most ([Deployment](../models/deployment.md#invariants)).

## The team

- **Header**: the team's name (600 16px), a link to its [Team page](./team-page.md); under it `deployed <time>` (13.5px `--tx4`). No goal here: it is the project's, in its wiki ([Session prompt](../architecture/session-prompt.md)). On the right, **`Deploy another team`** (with no team: **`Deploy a team`**), which opens [Deploy a team](./deploy-team.md) on this project (with no team, the team that worked here last chosen at first), and **`Stop`** in the danger style, led by the stop icon, which stops the team after the same confirmation as the Team page's ([Team page → Deployments](./team-page.md#deployments)).
- **One panel, a row per agent** (padding `12px 16px`, rows split by `--line`): first the team's agents, the lead first then the team file's order; then any worktree an earlier team left on the project (Stop keeps worktrees), by agent id.
  - Avatar 26, with its status dot while it is on the team.
  - Its name (500) and the `lead` tag, then `<Team> · <role>` (12.5px `--tx4`).
  - What it is doing, its [activity line](../architecture/harness-adapters.md#activity-lines) and when (Mono 12px `--tx3`), while there is one. Then a line per repo: the repo (Mono 12px `--tx4`), the branch checked out there (Mono 12.5px `--accTx`), `N ahead` when it has commits its repo's main hasn't, and `uncommitted changes` (`--accTx`) when the worktree holds changes not committed yet. Each line opens [Repos](./repos.md) on that repo and branch. A team agent with no worktree yet says `No worktree yet: made at its first start.`
  - **`Session`** for the team's agents (opens it in the [Session](./sessions.md) section), else `not on the team` (`--tx4`).
- **Empty** (no team, no worktree left): `No team is on <project>. Deploy one and each member gets its own worktree of the project's repos.`

Where a branch is checked out comes from git (`git worktree list`), on `GET /api/projects/:id/repos`: each branch's `agent` is the agent whose sandbox holds that worktree ([Repos → Data](./repos.md#data)). Removing a stopped team's worktrees is not built yet ([Remove a stopped deployment's worktrees](../../backlog/remove-worktrees.md)).

## In flight, then Merged

The [project pipeline](./project-pipeline.md)'s **In flight** and **Merged** sections: the work moving towards `main`, and what merged in the last 7 days. Side by side, half the width each, when both fit at 440px or more (the CEO, 2026-10-17); stacked otherwise, and either one alone takes the whole width.

Every branch in them is a link to its code in Repos: a merge request's in its repo; a work item's (which names no repo) in the first repo that has the branch with commits beyond its main, else the first that has it at all. A merge request's **Review** opens its review in the [Inbox](./reviews.md).

## Limits

- **Committed work only**: Repos reads what a branch has committed; an agent's uncommitted changes show only as the `uncommitted changes` flag.
