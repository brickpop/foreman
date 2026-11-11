---
type: spec
status: built
tags: [feature, ui, git, pipeline]
---

# Project team

A project's **Team** section ([Project page](./project-page.md)): the team on the project, its agents with what each is doing and the branch it has checked out, its [activity](./project-activity.md), then the work on its way to the CEO's branches and what merged. It was two sections, Deployment and Activity, until 2026-11-03 (the CEO: both are the team at work); `/deployment` and `/activity` links open their view of it. It replaced the Worktrees tab and Project settings' "Teams on this project" on 2026-10-31, both of which listed the same agents (the CEO: remove redundant info). A project has one team at most ([Deployment](../models/deployment.md#invariants)).

## The team

- **Header**: the team's name (600 16px), a link to its [Team page](./team-page.md); under it `deployed <time>` (13.5px `--tx4`). No goal here: it is the project's, in its wiki ([Session prompt](../architecture/session-prompt.md)). On the right, **`Deploy another team`** (with no team: **`Deploy a team`**), which opens [Deploy a team](./deploy-team.md) on this project (with no team, the team that worked here last chosen at first), **`Reload`** (led by the reload icon), which restarts the agents running on an older prompt on the new one, continuing their conversations, disabled while an agent is busy, with a line under the header saying which; always shown while a team is on the project, disabled with `Every agent runs on the current prompt.` when nothing is behind (the CEO, 2026-11-03) ([Team definition → Reload](../models/team-definition.md#reload)), and **`Stop`** in the danger style, led by the stop icon, which stops the team after the same confirmation as the Team page's ([Team page → Deployments](./team-page.md#deployments)).
- **A selector** under the header (the CEO, 2026-11-02, so each list has the whole width and the page stops growing): **`Agents · Activity · In flight · Merged`**, one shown at a time, in the [segmented](../design/design-system.md#buttons) style; in the URL as `?view=activity|inflight|merged`. With none in the URL, Agents while there are agents to show (the team's, or worktrees an earlier team left), else In flight.

### Agents

- **One panel, a row per agent of the team** (padding `12px 16px`, rows split by `--line`), the lead first, then the team file's order.
  - An 8px status dot while it is on the team (a worktree left by an earlier team has none).
  - Its name (500) and, for the lead, the ["lead" tag](../design/design-system.md#chips-and-badges) unless its name says it already, then `<Team> · <role>` (12.5px `--tx4`).
  - What it is doing, its [activity line](../architecture/harness-adapters.md#activity-lines) and when (Mono 12px `--tx3`), while there is one. Then a line per repo: the repo (Mono 12px `--tx4`), the branch checked out there (Mono 12.5px `--accTx`), `N ahead` when it has commits its repo's default branch hasn't, and `uncommitted changes` (`--accTx`) when the worktree holds changes not committed yet. Each line opens [Repos](./repos.md) on that repo and branch. A team agent with no worktree yet says `No worktree yet: made at its first start.`
  - **`Session`** for the team's agents (opens it in the [Session](./sessions.md) section), else `not on the team` (`--tx4`).
- **Empty** (no team): `No team is on <project>. Deploy one and each member gets its own worktree of the project's repos.`

Where a branch is checked out comes from git (`git worktree list`), on `GET /api/projects/:id/repos`: each branch's `agent` is the agent whose sandbox holds that worktree ([Repos → Data](./repos.md#data)).

### Earlier teams

Apart from the team's (the CEO, 2026-11-02): **`Left by earlier teams`** ([section title](../design/design-system.md#type), with their count), `Their worktrees stay after a Stop, so a redeploy picks them up.` (13px `--tx3`), and **`Clean up`** on the right; then a panel of their rows, as above. A row is a worktree an earlier team left on the project (Stop keeps worktrees) that no agent of the team holds: one of the same id takes it over ([Team definition](../models/team-definition.md#schema)). Each is named as the team that stopped there last has it, else by its agent id.

**Clean up** asks first (`Clean up what earlier teams left?`, `Removes their worktrees that hold nothing uncommitted, and their wt/<project>/ branches whose work is all merged (kept aside a while, recoverable by hand). Everything else stays, and is listed with why.`, **`Clean up`** in the danger style), then `POST /api/projects/:id/cleanup` removes **only what is safe** (the CEO, 2026-11-02), in each code repo, of every agent with a worktree in the project's sandboxes that doesn't run on this project (one running on another is cleaned here too: its branches there carry that project):

- its **worktree**, when it holds nothing not committed (git refuses otherwise) and no merge is under way (`git worktree remove`, its folder included; its branch stays);
- its **`wt/<project>/<agent>/*` branches** whose work is all merged ([Sandboxes and git policy → Cleaning up branches](../architecture/sandboxes-and-git.md#cleaning-up-branches)), and that nothing has checked out. Each head is kept under `refs/foreman/cleaned/<time>/` until backups are pruned ([Resume and backups](../../backlog/archive/resume-and-clean-desks.md)). A lead's `lead/<project>/<team>/*` branches stay: they are a review's.

It answers `{removed, kept}`, each a list of `{agent, repo, what (worktree | branch), branch, why}`; a toast says how many went, and what was kept is listed under the panel with why (`changes not committed`, `a merge is under way in it`, `not all of it is merged`, `it is checked out`). Nothing else changes: the agents' harness folders (their conversations) and sandbox folders stay.

## Activity

The team's messages and git actions, with the map of who calls whom, then the earlier teams' traffic: [Project activity](./project-activity.md).

## In flight and Merged

The [project pipeline](./project-pipeline.md)'s **In flight** and **Merged**: the work moving towards the CEO's branches, and what merged in the last 7 days (each with a calm line when empty: `No work in flight. It shows here from a hand-off to the lead to its merge.`, `Nothing merged in the last 7 days.`).

Every branch in them is a link to its code in Repos: a merge request's in its repo; a work item's (which names no repo) in the first repo that has the branch with commits beyond its default branch, else the first that has it at all. A merge request's **Review** opens its review in the [Inbox](./reviews.md).

## Limits

- **Committed work only**: Repos reads what a branch has committed; an agent's uncommitted changes show only as the `uncommitted changes` flag.
