---
type: spec
status: built
tags: [feature, ui, git]
---

# Repos

A project's **Repos** section ([Project page](./project-page.md); Repositories until 2026-10-31): the project's repos, their files and history by branch, read-only and as committed. Decided by the CEO on 2026-10-17 ([Repositories card](../../backlog/archive/project-code-tab.md)); only the repos since 2026-10-21, when the work in flight and what merged moved to [Deployment](./project-deployment.md) (the CEO). Body padding `24px 24px 48px`.

## The repos

A column of repos and branches (180–240px) beside the browser.

- **In the URL:** the repo and branch shown are the page's `repo` and `branch` (`/projects/ledger/repos?repo=api&branch=wt/tove/LED-44`), so a link opens them: an agent's worktree or a branch in flight on [Deployment](./project-deployment.md), a review's branch ([Inbox](./reviews.md)). Choosing a repo or a branch replaces them (no new history entry). The branch shown scrolls into the column's view when it changes (a link to one far down the list), never on a refresh. A repo the project doesn't have falls back to the first repo; a branch the repo doesn't have, to its first branch.
- **A repo picker** (the CEO, 2026-10-18): a dropdown of the repos (`<repo>/`, `· project repo` for the project repo, its branch count); choosing one shows its branches, its first one selected. The branches scroll within the column; the section fills the window's height (at least 360px), and the browser beside it scrolls within it too: a folder, a file (its `← Folder` bar staying in view) or the history.
- **The repos**: each code repo, then the **project repo** (`project/`) last, in the picker. **The branches**: `main` first, then the most recently committed first (the CEO, 2026-10-17; same-second ties by name). A branch: its name (Mono 12.5px), and under it `default branch` for `main` (the project's `main`, the branch merges go into), whose it is (`wt/<agent>/…` that agent's, `lead/<team>/…` that team's lead's), `N ahead of <main>` when it has commits `main` hasn't, and `uncommitted changes` (`--accTx`) when the worktree it is checked out in has changes not committed yet. The selected branch is on `--accSoft`.
- **The browser** shows the chosen branch (the first repo's `main` when the URL names none), with **Files** and **History** (segmented) on the right, and on the left, in Files, the breadcrumbs (the CEO, 2026-10-17):
  - **Files**: the breadcrumbs (`<repo> / src / lib`), then the folder: folders first (`name/`), then files with their size; a symlink says `link`, a submodule `submodule` (not opened). A file shows with `← Folder`, its size, and its text in monospace with line numbers (not selected with the text). A Markdown file is rendered, with **Rendered / Source**: inert, so raw HTML stays off, a web link opens in a new tab, a relative link is text, and an image shows its text, never loaded (agents write these files). A binary file (a NUL byte) or one over 1 MB says so instead. An empty folder: `Nothing committed here yet.`
  - **History**: the branch's commits, children before parents (`git log --topo-order`), 50 at a time with **Show more**: the short hash, the subject (`merge` beside a merge) with, as a second line in `--accTx` Mono, the branches pointing at that commit (the CEO, 2026-10-17; never `HEAD` or a tag), the author, the time (local, with the date when not today).
- **As committed:** files and history are what the branch has committed, never a worktree's files; the `uncommitted changes` flag is the only sign of work in progress. It refreshes when an agent commits, switches or merges (`team.action`), on `pipeline.changed`, and when projects or their repos appear, go or change (`projects.changed`).
- Diffs stay with the merge requests ([Project pipeline](./project-pipeline.md)).

## Data

`GET /api/projects/:id/repos` lists the repos and branches, each branch with `agent`: whose worktree has it checked out (the agent whose sandbox, `<workspace>/agents/<agent>/`, holds that worktree; `""` for the CEO's checkout or none); `…/repos/:repo/tree?ref=&path=`, `…/file?ref=&path=` and `…/log?ref=&skip=` read at a ref ([API](../architecture/api.md)). `:repo` is one of the project's code repos or `project`; anything else is `404`. A ref must name a commit and never starts with `-`; a path is relative and can't climb out with `..`; git then resolves it inside that commit's tree, so nothing outside the repo is reachable. The reads are the Git block's (`Resolve`, `Tree`, `File`, `Log`, `Worktrees`), composed by the repos block (`internal/repos`).
