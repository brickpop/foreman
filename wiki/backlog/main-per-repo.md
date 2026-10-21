---
type: task
status: done
priority: high
tags: [feature, bug]
epic: git
---

# A main branch per repo, and why an agent didn't start

**Why:** the CEO's first real project (2026-10-20) had a code repo whose main branch isn't `main`. The lead's start failed with "has no branch main to start lead/…/integration from", seen only in the server log; the pane said "crashed twice, or couldn't start". Three flaws: the reason never reaches the UI; adding a repo never checks the branch agents start from; and `main` is one name per project (`project.toml`) while code repos come from anywhere, so a project can't mix a repo on `main` with one on `master`.

## Decided (the CEO, 2026-10-21)

- **Each code repo has its own main branch:** `main` when the repo has one; otherwise the add dialog asks which branch is its default (its checkout's preselected). A new empty repo is made on `main`. Project settings changes it, to a branch the repo has. `main` leaves `project.toml`; the project repo stays on `main` (Foreman makes it).
- **An agent that can't start shows why** in its stopped pane, in place of the generic line; the next start clears it. A crash loop says so too.

## Steps

- [x] store: `project_repos.main` and `instances.stop_reason` (migration 004; rows from before get `main`)
- [x] home: the main at add (main, or the CEO's pick), `RepoBranches`, `SetRepoMain`, `main` out of `ProjectConfig`
- [x] every reader of the project's main reads the repo's: worktree start, switch, handoff, request_merge, merges and their notices, the repo browser, the prompt
- [x] the instance's stop reason: set on a failed start and a crash loop, cleared on start, shown in the pane
- [x] API: `PUT /api/projects/:id/repos/:repo/main`; UI: Project settings → Code repos picks it; Dashboard and Pipeline name the repo's main
- [x] specs: Project config, Project settings, Add a project, Instance, Sessions, API, Session prompt

**Done when:** a repo on `master` can be added and its agents start from it, beside a repo on `main` in the same project; a start that fails shows its reason in the pane; tests cover the add (main by default, a pick needed without one, an unknown one refused), the change (unknown branch refused), the per-repo base everywhere it is used, and the stop reason set and cleared.

**How it came out:** `project_repos.main` and `instances.stop_reason` (migration 004; repos from before read `main`). `GET /api/folders/branches` feeds the add dialog's pick, `PUT /api/projects/:id/repos/:repo/main` the Project settings select, which also flags a main the repo no longer has. Everything that read the project's `main` reads the repo's (`Project.Main`): worktree start, switch, handoff, request_merge, Merge to main, the review and main-moved notices, the repo browser, the prompt (one name when the repos share it, else each). `main` in `project.toml` is now a problem naming what replaced it, which blocks deploys until the line goes. A failed start stops the instance with `couldn't start: <why>`, a crash loop with `it exited twice in a row (code <n>)`, shown in the pane and cleared on the next start or any other status change. The workflow tests run with `web` on `trunk`, so every git tool is checked against a repo whose main isn't `main`.
