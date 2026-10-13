---
type: spec
status: planned
tags: [architecture, git, security, agents, pipeline]
---

# Sandboxes and git policy

Each [instance](../models/instance.md) works in its **own clone** of the project's repos, never in a shared working tree, and only a team lead's branches (`lead/<team>/*`) can become PRs to `main`. The sandbox manager creates, refreshes and polices those clones. The [session runner](./session-runner.md) mounts them into the instance's container.

## Layout and branches

- Sandbox: `<project>/.sandboxes/<agent>/<repo>/` for each repo of the [project](../models/project-config.md) ([Workspace](../models/workspace.md)), created with `git clone --reference <mirror> --dissociate` from Foreman's mirror of the repo: fast like a shared clone, then fully independent. (`--shared`, a plain `--reference` and worktrees all keep pointing at the source object store by host path, which isn't mounted in the container, so git would break inside it.) Agent ids are workspace-unique, so sandbox paths never collide ([Team definition](../models/team-definition.md#schema)).
- `.sandboxes/` must be git-ignored in the project folder: Foreman adds it to `.git/info/exclude` of any enclosing repo.
- **Worktree branch** `wt/<agent>/<card>`: where a member does the work for one card.
- **Lead branches** `lead/<team>/<topic>` (e.g. `lead/atlas/settlement-v2`): where the lead integrates handoffs, any number of them; `lead/<team>/integration` is just the default topic created at deploy. They are the only source of PRs to `main`, one open PR per lead branch at a time. The team is part of the name because two teams can be deployed on the same project.
- `main` (name from `project.yaml` `main`) changes **only by the CEO's merge**, on the forge or with git; Foreman never merges ([Pipeline and PRs](../features/pipeline-and-prs.md#review-and-merge)).

On deploy ([Agent runtime](./agent-runtime.md#instance-lifecycle)): the lead checks out `lead/<team>/integration` (created from `main` if missing); members idle on `main` and create `wt/<agent>/<card>` when assigned a card. Sandboxes fetch the new `main` at their next idle after a merge. Stopping a deployment keeps sandboxes and worktrees on disk.

## Branch policy

| Who | May publish |
|---|---|
| Member | `wt/<self>/*` only, fast-forward only |
| Lead | `wt/<self>/*` and `lead/<team>/*`; a `lead/<team>/*` branch may also be rewritten (non-fast-forward), so the lead can rebase it onto a newer `main` |
| Nobody | `main`: it changes only by the CEO's merge |

Pushes to the **forge** (for PRs) are done by Foreman, not the agents, using the workspace token ([Forge adapters](./forge-adapters.md)). Agents never receive forge tokens.

### Where the policy is enforced

The PRD asks for "a pre-receive hook (or Foreman's own push proxy)" on "the project repo as the single remote". A hook only protects if it lives where agents can't edit it: hooks inside an agent's own sandbox are under the agent's control, and the CEO's project repo is not mounted into any container. So:

- Foreman keeps a **bare mirror per repo** (Foreman-owned, under `.foreman/`, never mounted into containers). It is the only place sandbox branches are published to, and Foreman applies the policy table above when it accepts a branch.
- **Foreman never touches the CEO's working copy.** The mirror fetches `main` from the forge when there is one, otherwise from the CEO's project repo; the CEO pulls `main` themselves. The only thing Foreman ever writes into the project repo is the lead branch of a project without a forge, as a plain branch for the CEO to review ([Forge adapters](./forge-adapters.md)); `wt/*` branches never reach it.
- **Agents never push.** On `handoff` and `open_pr`, Foreman fetches the named branch out of the caller's sandbox into the mirror, checking it against the policy table. Calling `open_pr` again on a lead branch whose PR is still open is how a lead updates it (after a rebase, or new integrations): Foreman fetches the branch again, updates the PR's branch on the forge (a force update when it was rebased) and reruns the checks. On a handoff Foreman also fetches the branch into the lead's sandbox (as `refs/remotes/foreman/wt/<agent>/<card>`), so the lead can review and merge it without any access to the mirror or to the member's sandbox. Nothing to authenticate, nothing to proxy.
- **Pruning:** merged `wt/*` refs are pruned from the mirror automatically. Sandboxes are deleted only on an explicit CEO action (stopping a deployment keeps them).

## Wiki access

Only the scrum master's instance has the project `wiki/` writable; others get it read-only. If the team has no scrum master, the lead gets write access. With containers this is enforced by the mount mode ([Session runner](./session-runner.md#podman-runner-production)), not by convention.

## Checks

`checks` from `project.yaml` (e.g. `npm test`) run "in a sandbox before handoff and on PR". The container image must contain the project's toolchain for this to work ([Agent images](./tech-stack.md#agent-images): the project layer carries the toolchain).

Foreman runs them itself, in a **throwaway container** from the project's image on a clean checkout of the branch: before a handoff is accepted (the member's branch) and on `open_pr` (the lead branch). A clean checkout makes the result reproducible and immune to local state in a sandbox. The result is the `checks_status` of the [work item](../models/work-item.md) (handoff) or of the [pull request](../models/pull-request.md) (`open_pr`); when a forge is configured, its CI statuses are shown alongside ([Forge adapters](./forge-adapters.md)).
