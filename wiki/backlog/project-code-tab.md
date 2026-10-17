---
type: task
status: done
priority: medium
tags: [feature]
epic: git
---

# Project → Repositories: repos, branches, files, history

**Why:** the CEO, 2026-10-16. A project's page doesn't show what it holds: its code repos, the agents' branches and worktrees, the files. The CEO expects to open a project and see that, simply.

**The CEO's direction (2026-10-16):** a **Code** tab, read-only. Possibly also a **Git** tab with the history and branches. Needs refinement.

**Starting point** (proposed):
- **Code:** each code repo with its branches: `main`, and each agent's branch, saying whose it is, how many commits it's ahead of `main`, and whether its worktree has uncommitted changes. Clicking a branch browses its files, read-only. Diffs stay in Changes.
- **Git** (separate, or a section of Code): the history of `main` and of a chosen branch (commit, author, date, message), and the merges.

**To decide when the card starts:** one tab or two; whether the project repo (`project/`, the wiki) shows too; file view only, or rendered Markdown; and how much is live (the worktrees change under the CEO's eyes).

**Decided (the CEO, 2026-10-17):**
- **Tabs become `Overview · Board · Repositories · Activity · Settings`.** (the CEO named it Repositories rather than Repo.) Changes goes as a tab.
- **Overview opens with Needs you** (the merge requests and requests waiting on the CEO, with Merge / Create PR / Reject), only when there is something; the wiki reader follows. The open-requests badge moves to Overview. An old `?tab=changes` link lands on Overview.
- **Repositories** (one tab: code and git): each code repo, then the project repo (`project/`) last, all read-only.
  - **In flight** and **Merged** (from Changes) move here: they are git facts.
  - Each repo's branches: `main`, then each agent's (whose, commits ahead of `main`, `uncommitted changes` when its worktree has some).
  - A chosen branch shows **Files** (a tree, then a file: monospace with line numbers; Markdown rendered, with a toggle to the source; binary and very large files say so) and **History** (commits: short hash, author, date, subject; merges marked), paged.
  - **Committed state only**, plus the uncommitted flag; it refreshes when an agent commits, switches or merges (`team.action`), and on `pipeline.changed`.
  - Diffs stay with the merge requests.

**Design (2026-10-17):**
- Git block reads (behind the existing Git interface): branches with ahead counts, a worktree's cleanliness, `ls-tree` at a ref and path, `show` of a file at a ref (with a size cap and a binary check), `log` of a ref, paged.
- API: `GET /api/projects/:id/repos` (repos and branches), `…/repos/:repo/tree?ref=&path=`, `…/repos/:repo/file?ref=&path=`, `…/repos/:repo/log?ref=&before=`. `:repo` must be one of the project's repos or `project`; a ref must exist (`rev-parse --verify`, never starting with `-`); a path stays inside the repo (git resolves it in the ref's tree, so `..` can't leave it).

- [x] Overview with Needs you; Changes' sections moved; links to `tab=changes` (Home, the Activity panel's "Review in Changes") point to the new places
- [x] Git reads and their tests (refs and paths that try to escape included)
- [x] the four routes and their tests
- [x] the Repositories tab: In flight, Merged, branches, Files, History
- [x] specs: Wiki reader (tabs, Needs you), Changes (folded into Overview and Repositories), a Repositories spec, API, Home

**Built 2026-10-17** ([Repositories](../product/features/repositories.md), [Project pipeline](../product/features/project-pipeline.md)): the git reads (`internal/git/read.go`), the repos block (`internal/repos`), four routes, the tab, Needs you on Overview, Changes gone. Checked end to end on the binary: a branch's ahead count and uncommitted flag, files, history, and four escape attempts refused (`../`, `../../etc` as a repo, `--all` as a ref, `.sandboxes` as a repo). A repo whose default branch isn't the project's `main` (a `master` clone) lists without ahead counts rather than failing.
