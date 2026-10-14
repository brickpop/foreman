---
type: spec
status: planned
tags: [architecture, git, security, agents, pipeline]
---

# Sandboxes and git policy

Each [instance](../models/instance.md) works in its **own git worktree** of each code repo of the project, on its own branch, never in the CEO's checkout. Only a team lead's branches (`lead/<team>/*`) can be requested for merging into `main`, and only the CEO's action merges them. The [Git block](./building-blocks.md#8-git-git) creates the worktrees on the host; the [session runner](./session-runner.md) mounts them into the instance's container. **Git belongs to Foreman: agents only read it.** Every git write (a commit, a merge, a new branch) is a team tool that Foreman runs on the host ([below](#agents-and-git)). The project's knowledge base is different: one live checkout of the [project repo](../models/project-config.md), shared by every agent ([below](#project-knowledge-base)).

**Phasing:** worktrees and branches per agent arrive in [Terminal multiplexer](../../backlog/epics/terminal-multiplexer.md) (epic 1); the `switch`, `commit` and `update` tools, and hand-offs that name a branch, in [Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md) (they need the agent link). **Decided: in epic 1 agents can't commit**: the CEO drives each session by hand and looks at the uncommitted work in each worktree; integration detection, checks and pushes to the forge in the [Git epic](../../backlog/epics/git.md).

## Layout

- **Worktree:** `<project>/.sandboxes/<agent>/<repo>/` for each code repo of the [project](../models/workspace.md#discovery), created on the host with `git -C <project>/<repo> worktree add`. A worktree shares its repo's object store and refs: nothing is copied, and every branch any agent creates is at once visible to the others and to the CEO. Agent ids are workspace-unique, so worktree paths never collide ([Team definition](../models/team-definition.md#schema)).
- `<project>/.sandboxes/<agent>/` is the instance's **sandbox**: its working directory, holding its worktrees and its view of the project repo (`project/`, mounted there).
- `.sandboxes/` must be git-ignored: Foreman adds it to `.git/info/exclude` of any enclosing repo.
- The project repo `<project>/project/` never gets agent worktrees: it is the live, shared knowledge base.

## Branches

- **Home branch** `wt/<agent>/main`: each member's own starting point (the lead's is `lead/<team>/integration`: the lead never has `wt/` branches) in a repo, created from `main` when its worktree is first made. Git lets a branch be checked out in only one worktree at a time, so no agent checks out `main` itself; members idle here.
- **Topic branch** `wt/<agent>/<topic>` (e.g. `wt/tove/LED-44`, the topic usually a card id): where a member does one piece of work. Agents can't create branches themselves: `switch(branch)` has Foreman switch the agent's worktrees to it, creating it from `main` the first time ([Agents and git](#agents-and-git)).
- **Lead branches** `lead/<team>/<topic>` (e.g. `lead/atlas/settlement-v2`): where the lead integrates hand-offs, any number of them, each reached (and made the first time) with `switch(branch)`; `lead/<team>/integration` is the default topic, created from `main` at deploy and checked out in the lead's worktree. They are the only source of merge requests to `main`, one open merge request per lead branch at a time. The team is part of the name because two teams can be deployed on the same project.
- `main` (name from `project.toml` `main`) changes **only by the CEO's decision**: Merge to main (Foreman merges on the CEO's click), a PR merged on the forge, or the CEO's own git; Foreman never merges on its own ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

On deploy ([Agent runtime](./agent-runtime.md#instance-lifecycle)), `Git.Prepare` makes each missing worktree (an existing one is reused as it is). Since every worktree shares the repo, a new `main` is visible to all of them as soon as the repo has it. Stopping a deployment keeps the worktrees on disk; they are removed (`git worktree remove`) only on an explicit CEO action. **Branches are cleaned up:** once a deployment ends, Foreman deletes each of its agents' `wt/*` branches that is merged into `main` (`Git.Prune`); unmerged ones stay. A branch still checked out in a kept worktree can't be deleted, so it goes when that worktree is removed.

## Mounts

Everything at the **same path as on the host** ([Session runner](./session-runner.md#podman-engine-production)), except the project repo:

| Host path | In the container | Mode |
|---|---|---|
| `<project>/.sandboxes/<agent>/` (the sandbox: its worktrees) | same path; the working directory | rw |
| `<project>/<repo>/.git` (each code repo) | same path | **ro** |
| `<project>/project/` (the live project repo) | `<project>/.sandboxes/<agent>/project` | rw for the lead, ro for the others |
| `<project>/project/.git` | `<project>/.sandboxes/<agent>/project/.git`, over the mount above | **ro** for every agent |

A worktree's `.git` is a file pointing at `<project>/<repo>/.git/worktrees/<name>` by host path, so git works inside the container only because the repo's `.git` is mounted at that same path. The CEO's working files (`<project>/<repo>/` outside `.git`) are never mounted, and no agent ever touches the CEO's checkout or checks out a branch there (how Foreman's own Merge to main relates to it is [open](../features/pipeline-and-merges.md#open-questions)).

## Agents and git

**Agents read git; Foreman writes it.** Every repo's `.git`, the project repo's included, is mounted read-only, so inside the container `git status`, `diff`, `log`, `blame` and `show` work, and every git write (`commit`, `checkout`, `branch`, `merge`, `rebase`, `push`) fails. Writes go through [team tools](./team-tools.md#tools), which Foreman runs on the host in the caller's own worktrees:

- **`switch(branch)`**: switches the agent's worktrees in each of its code repos to one of its own branches (`wt/<agent>/<topic>`, or `lead/<team>/<topic>` for the lead), creating it from `main` if it doesn't exist yet; so an agent can also go back to earlier work. A lead can keep several lead branches, and so several merge requests, at once.
- **`commit(message)`**: stages everything in each of the agent's worktrees that has changes and commits it on the branch checked out there, authored as the agent. For the lead it also commits its changes in the live project repo.
- **`update(branch)`**: merges a named branch (the lead's, `main`, a teammate's) into the agent's worktrees. On a conflict the markers stay in the files; the agent resolves them and calls `commit`, which concludes the merge.
- **`handoff`** commits pending work first, so a hand-off never misses the last changes.

Foreman runs one git command at a time per repo, so git's own lock never collides, and its git runs with the repo's hooks and fsmonitor turned off, so nothing configured in a repo runs a program on the host when Foreman acts. Its commits are unsigned, even where the CEO's git signs by default: they are automatic, and a signing agent would prompt, or fail, on every one. The [session prompt](./session-prompt.md) tells agents that git writes go through the tools.

What this buys: no agent can damage refs, or plant a hook or a `.git/config` setting that would run a program on the CEO's machine the next time git runs there; every commit is authored as the agent who made it; and branch rules hold by construction, since Foreman is the only writer. **Trade-off, accepted:** every agent can still read every branch of the repo, its teammates' included.

**Later option, not built:** hide git from agents entirely, mounting only the worktree and no `.git`. Agents would no longer see other branches, at the cost of `diff`, `log` and `blame` inside the container, unless Foreman offered them as tools.

## Hand-offs

A hand-off is a message naming a branch: `handoff(mara, "wt/tove/LED-44", summary)`. Every worktree shares the repo, so the lead can review the branch at once (`git diff` in its own worktree) and merge it with `update`; nothing is fetched or copied, and agents never push anywhere.

## Branch policy

| Who | May hand off or request a merge of |
|---|---|
| Member | `wt/<self>/*` only |
| Lead | `lead/<team>/*` only: the lead works only on its lead branches, never on `wt/` ones; it catches up with a newer `main` by merging it in (`update("main")`), never by rewriting the branch |
| Nobody | `main`: it changes only by the CEO's merge |

### Where the policy is enforced

By Foreman, never by hooks in the repo: agents can't write `.git` at all, and Foreman is the only one that writes it, through the tools:

- **`switch`**, **`commit`** and **`update`** act only on the caller's own worktrees and the branches checked out there; **`handoff`** accepts only `wt/<self>/*`; **`request_merge`** only the caller's own `lead/<team>/*` ([Protocol → Team tools](./protocol.md#team-tools)).
- **Agents never push, and nothing is pushed on its own.** Only the CEO's **Create PR** on a merge request makes Foreman push the lead branch to the **forge** from the host, with the workspace token ([Forge adapters](./forge-adapters.md)); agents never receive forge tokens. Calling `request_merge` again on a lead branch whose merge request is still open refreshes it (after merging `main` in, or new integrations): Foreman reruns the checks, and pushes again if it has a PR. Lead branches only ever move forward: nothing is rebased.
- **Merge to main** is Foreman merging the branch into `main` on the host, on the CEO's click; with or without a forge ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

## Project knowledge base

The [project repo](../models/project-config.md) `<project>/project/` (`project.toml` and the wiki) is **one live checkout**, mounted into every container of the project: read-write for the **lead** (the lead of each team deployed on the project), read-only for everyone else, enforced by the mount mode. Its `.git` is read-only for every agent, the lead included: the lead edits the files and calls `commit()`, which commits on the project repo's `main` directly: no branches, no merge requests. Everyone else contributes through the [`contribute`](./team-tools.md#tools) team tool: Foreman writes the contribution into `project/wiki/inbox/` and tells the lead. There is no writable folder for read-only agents.

**Several leads, one checkout.** Each team deployed on the project has its own lead, and they all share this one checkout. A lead's `commit()` takes everything changed in it, so it can sweep another lead's unfinished edit into its own commit: nothing is lost, only the attribution suffers. Accepted for v1; a worktree per lead, with Foreman merging each commit into `main`, is an option for later ([backlog](../../backlog/index.md#not-yet-scheduled)).

**Foreman commits its own writes** to the project repo, one commit per write, like the [workspace repo](../models/workspace.md#the-workspace-repo)'s one commit per save: a `contribute` note authored as the contributing agent, a checkbox tick or card move from the UI authored as the CEO. Each commit holds only the file Foreman wrote, so the lead's uncommitted edits in the same checkout are left alone.

**Changes to `project.toml` that need review** (the Chief of Staff's) are merge requests like any other: Merge to main, Create PR on the project repo's forge (`forge.project_repo` in [Project config](../models/project-config.md#schema)), or Reject. Nothing in the project repo is pushed on its own: the CEO pushes its `main` with **Sync** in Project settings ([Forge adapters → Sync](./forge-adapters.md#sync-the-reserved-repos)) ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

## Checks

`checks` from `project.toml` (e.g. `npm test`) run "in a sandbox before handoff and on a merge request". The container image must contain the project's toolchain for this to work ([Agent images](./tech-stack.md#agent-images): the project layer carries the toolchain).

Foreman runs them itself, in a **throwaway container** from the project's image on a clean checkout of the branch: before a handoff is accepted (the member's branch) and on `request_merge` (the lead branch). A clean checkout makes the result reproducible and immune to local state in a worktree. The result is the `checks_status` of the [work item](../models/work-item.md) (handoff) or of the [merge request](../models/merge-request.md) (`request_merge`); when a forge is configured, its CI statuses are shown alongside ([Forge adapters](./forge-adapters.md)).
