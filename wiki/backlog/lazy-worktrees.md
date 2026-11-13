---
type: task
status: done
priority: high
tags: [feature]
epic: git
---

# Open repos on demand; idle worktrees detached

**Why:** the CEO, 2026-11-12. Every agent start made, per agent and per repo of the project, a home branch, a worktree and a checkout of every submodule, though most agents touch one or two repos. Decided: lazy repos, detached idle worktrees (no home branches), and implicit opening plus an explicit `open(repo)`. The design is in [Sandboxes and git policy → Opening a repo](../product/architecture/sandboxes-and-git.md#opening-a-repo) and [→ Branches](../product/architecture/sandboxes-and-git.md#branches).

Done means an agent start makes no worktree and no branch, and the tools behave as the spec says.

- [x] Git: make a worktree detached at the default branch (`Prepare` without a branch); `Freshen` moves a clean detached worktree whose commit the default branch holds
- [x] Launch: reuse open worktrees only (relink, freshen), no `Prepare` per repo; keep mounting every repo's `.git`. An agent with no team tools (no `open`) gets every repo, detached
- [x] `open(repo)` tool: on the branch the agent's other worktrees are on (made from the default when missing), else detached; then submodules; recorded as an action
- [x] Implicit opening: `switch` (repos that have the branch), `merge` (repos that have the branch), `set_submodule` (its repo); `switch` to a new branch with no repo open refuses
- [x] `commit`, `merge`, `set_submodule` refuse while detached: switch to a topic first
- [x] `freshenDesks` after a merge: detached worktrees, not home branches; drop `HomeBranch`/`IsHomeBranch` and the "home branch never goes" prune rule
- [x] Session prompt: no "you work on <home branch>"; says repos open on demand, how to read one unopened, and to switch to a topic before committing
- [x] Specs: team-tools, protocol (open tool, refusals), session-prompt, agent-runtime (start), project-team (worktree status of unopened repos)
- [x] Team section and Clean up see idle worktrees: `GET …/repos` gives each repo's `idle` agents; Clean up removes a clean idle worktree like any other
- [x] Tests for each of the above
