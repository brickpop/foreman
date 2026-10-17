---
type: spec
status: built
tags: [feature, ui, git]
---

# Repositories

A project's **Repositories** tab ([Wiki reader → Project header and tabs](./wiki-reader.md#project-header-and-tabs)): what the project holds, read-only and as committed. Decided by the CEO on 2026-10-17 ([Repositories card](../../backlog/project-code-tab.md)). Body padding `24px 48px 64px`, 36px between sections.

## In flight, then Merged

First the [project pipeline](./project-pipeline.md)'s **In flight** and **Merged** sections, as they were on Changes: the work moving towards `main`, and what merged in the last 7 days.

## The repos

Then **Repositories**: a column of repos and branches (180–240px) beside the browser.

- **Each code repo**, then the **project repo** (`project/`, labelled `project repo`) last: its name (Mono 600, `<repo>/`), then its branches, `main` first, then by name. A branch: its name (Mono 12.5px), and under it whose it is (`wt/<agent>/…` that agent's, `lead/<team>/…` that team's lead's), `N ahead of <main>` when it has commits `main` hasn't, and `uncommitted changes` (`--accTx`) when the worktree it is checked out in has changes not committed yet. The selected branch is on `--accSoft`.
- **The browser** shows the chosen branch (the first repo's `main` until one is picked), with **Files** and **History** (segmented):
  - **Files**: breadcrumbs (`<repo> / src / lib`), then the folder: folders first (`name/`), then files with their size; a symlink says `link`, a submodule `submodule` (not opened). A file shows with `← Folder`, its size, and its text in monospace with line numbers (not selected with the text). A Markdown file is rendered, with **Rendered / Source**: inert, so raw HTML stays off, a web link opens in a new tab, a relative link is text, and an image shows its text, never loaded (agents write these files). A binary file (a NUL byte) or one over 1 MB says so instead. An empty folder: `Nothing committed here yet.`
  - **History**: the branch's commits, children before parents (`git log --topo-order`), 50 at a time with **Show more**: the short hash, the subject (`merge` beside a merge), the author, the time (local, with the date when not today).
- **As committed:** files and history are what the branch has committed, never a worktree's files; the `uncommitted changes` flag is the only sign of work in progress. It refreshes when an agent commits, switches or merges (`team.action`), on `pipeline.changed`, and when repos appear (`workspace.changed`).
- Diffs stay with the merge requests ([Project pipeline](./project-pipeline.md)).

## Data

`GET /api/projects/:id/repos` lists the repos and branches; `…/repos/:repo/tree?ref=&path=`, `…/file?ref=&path=` and `…/log?ref=&skip=` read at a ref ([API](../architecture/api.md)). `:repo` is one of the project's code repos or `project`; anything else is `404`. A ref must name a commit and never starts with `-`; a path is relative and can't climb out with `..`; git then resolves it inside that commit's tree, so nothing outside the repo is reachable. The reads are the Git block's (`Resolve`, `Tree`, `File`, `Log`, `Worktrees`), composed by the repos block (`internal/repos`).
