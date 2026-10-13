---
type: spec
status: planned
tags: [architecture, git, security, agents, pipeline]
---

# Sandboxes and git policy

Each [instance](../models/instance.md) works in its **own clone** of the project's repos, never in a shared working tree, and only the team lead's integration branch can become a PR to `main`. The sandbox manager creates, refreshes and polices those clones. The [session runner](./session-runner.md) mounts them into the instance's container.

## Layout and branches

- Sandbox: `<project>/.sandboxes/<agent>/<repo>/` for each repo of the [project](../models/project-config.md) ([Workspace](../models/workspace.md)).
- `.sandboxes/` must be git-ignored in the project folder: Foreman adds it to `.git/info/exclude` of any enclosing repo.
- **Worktree branch** `wt/<agent>/<card>`: where a member does the work for one card.
- **Lead branch** `lead/*` (e.g. `lead/integration`): where the lead integrates handoffs; the only source of PRs to `main`.
- `main` (name from `project.yaml` `main`) changes **only by CEO merge** ([Pipeline and PRs](../features/pipeline-and-prs.md)).

On deploy ([Agent runtime](./agent-runtime.md#instance-lifecycle)): the lead checks out `lead/integration` (created from `main` if missing); members idle on `main` and create `wt/<agent>/<card>` when assigned a card. Sandboxes fetch the new `main` at their next idle after a merge. Stopping a deployment keeps sandboxes and worktrees on disk.

## Branch policy

| Who | May publish |
|---|---|
| Member | `wt/<self>/*` only |
| Lead | `wt/<self>/*` and `lead/*` |
| Nobody | `main`: it changes only by CEO merge |

Pushes to the **forge** (for PRs) are done by Foreman, not the agents, using the workspace token ([Forge adapters](./forge-adapters.md)). Agents never receive forge tokens.

### Where the policy is enforced

The PRD asks for "a pre-receive hook (or Foreman's own push proxy)" on "the project repo as the single remote". A hook only protects if it lives where agents can't edit it: hooks inside an agent's own sandbox are under the agent's control, and the CEO's project repo is not mounted into any container. **Recommendation:**

- Foreman keeps a **bare mirror per repo** (Foreman-owned, under `.foreman/`, never mounted into containers). It is the only place sandbox branches are published to, and Foreman applies the policy table above when it accepts a branch.
- The CEO's project repo is updated **only by Foreman** (fetching `main` after a merge), so it never fills up with `wt/*` branches.
- How branches travel from a sandbox to the mirror is open (question 2): either Foreman fetches the named branch out of the sandbox on `handoff` / `open_pr` (agents never push at all), or Foreman serves git over HTTP on the ACP listener with the instance token and enforces the policy per push.

## Wiki access

Only the scrum master's instance has the project `wiki/` writable; others get it read-only. If the team has no scrum master, the lead gets write access. With containers this is enforced by the mount mode ([Session runner](./session-runner.md#podman-runner-production)), not by convention.

## Checks

`checks` from `project.yaml` (e.g. `npm test`) run "in a sandbox before handoff and on PR". The container image must contain the project's toolchain for this to work ([Session runner](./session-runner.md#open-questions)).

## Open questions

1. **Sandbox strategy** (PRD §16.3): `git clone --shared` vs `git worktree` vs full clones (disk vs isolation). New constraint from containers: `--shared`, `--reference` and worktrees point at the source repo's object store by host path, which is not mounted in the container, so git breaks inside it. Options: full clones (simple, most disk); `clone --reference … --dissociate` (fast, then independent); mount the source object store read-only (saves disk, leaks every branch). Recommendation: `--reference --dissociate` from the Foreman mirror.
2. **Publishing transport.** Foreman fetches branches out of sandboxes on `handoff` / `open_pr` (no push at all, nothing to authenticate; lead receives handoffs as fetched refs) vs Foreman-served git smart HTTP with per-instance tokens (agents use plain `git push`/`fetch`, policy checked per ref). Recommendation: fetch on handoff in v1.
3. **Check runner and check status** (PRD §16.4): run checks in the lead's sandbox, a throwaway clone, or rely on forge CI? And with a forge, is the reported check status the forge's CI, Foreman's own run of `checks`, or both ([Forge adapters](./forge-adapters.md))? Recommendation: a throwaway container from the lead's image on a clean checkout of `lead/*` (reproducible, can't be influenced by local state), always run; forge CI statuses shown alongside when a forge is configured.
4. **Lead branch collisions.** Two teams deployed on the same project both use `lead/integration` in the same repo, so two leads would integrate on the same branch. Recommendation: `lead/<team>/integration` (leads may push `lead/<team>/*`, and PR branches are named per team).
5. **Disk growth.** Sandboxes and merged `wt/*` branches are kept forever. Who prunes merged worktree branches and sandboxes of stopped deployments? Recommendation: prune merged `wt/*` refs automatically; delete sandboxes only on explicit CEO action.
