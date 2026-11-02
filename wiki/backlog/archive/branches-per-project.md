---
type: task
status: done
priority: high
tags: [bug, feature]
epic: workflow
---

# Agents' branches carry the project

**Why:** evm, active on one project, couldn't start its lead (the CEO, 2026-11-03): "team evm also works on aragon-workspace, which shares the repo research-legacy …". aragon-workspace had no deployment active, only a stopped one. Agents' branches name the team or agent but not the project (`lead/evm/integration`, `wt/leady/main`), so one team on two projects sharing a repo would use the same branches in its `.git`. `sharing` (`internal/instances/launch.go`) refuses that, at every start and counting stopped deployments, since Stop keeps worktrees. The team is then stuck wherever the repo is shared, and **Clean up** can't free it either: it skips any agent running on *any* project. That's the debt [Projects as records: known edges → A stopped deployment holds its repos](../../technical-debt.md#projects-as-records-known-edges).

**Decided (the CEO, 2026-11-03):** the project goes into the branch names, so the same team works on any number of projects sharing a repo, and the refusal goes.

**Decided too (the CEO, 2026-11-03):**
- Names: `wt/<project>/<agent>/<topic>` (home `wt/<project>/<agent>/main`) and `lead/<project>/<team>/<topic>` (home `lead/<project>/<team>/integration`). Project first, so a repo's branches group by project. Project ids are slugs, safe in a ref.
- **No rename:** existing branches keep their old names. Agents start on the new home branches; work on old names stays visible in Repos, out of the review flow (Foreman isn't released).
- Agents continuing a conversation that remember old names: the prompt says the new ones (Reload), and the policy refuses an old name, naming the right one.

**What changes (found 2026-11-03):**
- `git.HomeBranch` takes the project; `IsHomeBranch` reads four parts.
- The own-branch policy (`tool.own`, `internal/workflow/workflow.go`) and every prefix built from it: `resume.go`, `withdraw.go`, `pipeline.go`, `merges.go` (request_merge, `freshenDesks`, which then freshens only this project's desks), `cleanup.go`.
- The tools' schema descriptions (`wt/<you>/<topic>`, `lead/<team>/<topic>`) and the prompt's branch lines (`internal/prompt/prompt.go`, `request_merge` in its tool list).
- `sharing` and its `samePath` removed; the `CANT_DEPLOY` case in [API](../../product/architecture/api.md) loses it.
- Clean up: an agent is skipped only while it runs on *this* project (its branches are this project's now).
- UI: `branchOwner` (`ui/src/app/repos.ts`) reads `wt/<project>/<agent>` and `lead/<project>/<team>`; Clean up's text names `wt/<project>/` branches.
- Unchanged: `home.isAgentBranch` (`wt/*`, `lead/*` still can't be targets or defaults), backups under `refs/foreman/` (they keep the name they had).
- Specs: [Sandboxes and git policy](../../product/architecture/sandboxes-and-git.md) (branch names; "Two projects, one repo" now allowed), [Project config](../../product/models/project-config.md) (drop the refusal), [Session prompt](../../product/architecture/session-prompt.md), [Protocol](../../product/architecture/protocol.md), [Project team → Earlier teams](../../product/features/project-team.md#earlier-teams), [Repos](../../product/features/repos.md); the debt entry above deleted.

- [x] names, policy, prompts, tools, Clean up, UI
- [x] `sharing` removed
- [x] tests: the same team deployed on two projects sharing a repo, each agent on its own branches, merges and Clean up of one never touching the other's; the policy refuses another project's branch; `IsHomeBranch` on the new shape and refuses the old; `branchOwner`
- [x] specs as built
