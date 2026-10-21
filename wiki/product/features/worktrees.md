---
type: spec
status: built
tags: [feature, ui, git, pipeline]
---

# Worktrees

A project's **Worktrees** tab, after Repositories ([Wiki reader → Project header and tabs](./wiki-reader.md#project-header-and-tabs)): what each agent has checked out, then the work on its way to `main` and what merged. Split from [Repositories](./repositories.md) on 2026-10-21, so that Repositories holds only the repos (the CEO). Body padding `24px 48px 64px`, 36px between sections.

## Agent worktrees

Label `Agent worktrees`, subtitle `the branch each agent has checked out`. One panel, a row per agent with a worktree on the project (an agent's [sandbox](../architecture/sandboxes-and-git.md) holds one worktree per code repo), by agent id; rows separated by a `--line` border, padding `12px 16px`:

- Avatar 26 with its status dot while its team is deployed.
- Its name (500) and the `lead` tag, then `<Team> · <role>` (12.5px `--tx4`).
- A line per repo: the repo (Mono 12px `--tx4`), the branch checked out there (Mono 12.5px `--accTx`), `N ahead` when it has commits its repo's main hasn't, and `uncommitted changes` (`--accTx`) when the worktree holds changes not committed yet. Each line is a link that opens [Repositories](./repositories.md) on that repo and branch, to read the work in progress as committed.
- **`Session`** while its team is deployed (opens it in [Sessions](./sessions.md)), else `not deployed` (`--tx4`).

Where a branch is checked out comes from git (`git worktree list`), on `GET /api/projects/:id/repos`: each branch's `agent` is the agent whose sandbox holds that worktree ([Repositories → Data](./repositories.md#data)). A worktree stays after its deployment stops (until the CEO removes it), so a stopped team's agents keep their rows.

**Empty**: `No agent has a worktree here yet: each gets one when its team is deployed.`

## In flight, then Merged

The [project pipeline](./project-pipeline.md)'s **In flight** and **Merged** sections, as they were on Repositories: the work moving towards `main`, and what merged in the last 7 days. Side by side, half the width each, when both fit at 440px or more (the CEO, 2026-10-17); stacked otherwise, and either one alone takes the whole width.

Every branch in them is a link to its code in Repositories: a merge request's in its repo; a work item's (which names no repo) in the first repo that has the branch with commits beyond its main, else the first that has it at all.

## Limits

- **Committed work only**: Repositories reads what a branch has committed; an agent's uncommitted changes show only as the `uncommitted changes` flag.
