---
type: spec
status: built
tags: [feature, ui, git]
---

# Repos

A project's **Repos** section ([Project page](./project-page.md); Repositories until 2026-10-31): the project's repos, listed and managed, each opening its own page with its files and history by branch, read-only and as committed. Decided by the CEO on 2026-10-17 ([Repositories card](../../backlog/archive/project-code-tab.md)); only the repos since 2026-10-21, when the work in flight and what merged moved to [Deployment](./project-deployment.md) (the CEO). Body padding `24px 24px 48px`.

## The list

`/projects/<project>/repos`, at most 920px wide: a grid of **cards** (at least 260px wide, 12px apart; the CEO, 2026-11-02, replacing a panel of rows), one per code repo of the [record](../models/project-config.md#the-project-record), then the **project repo**'s. The CEO moved them here from Settings on 2026-11-01: the repos are managed where they are read.

- **A card** (the Dashboard's card: `--panel`, `--line` border, radius 12, padding `14px 16px`) opens the repo's page wherever it is clicked (hover border `--line3`), except a repo with a problem, which can't be read. On it:
  - its **name as words** (600 15px; the CEO, 2026-11-02: not a path): hyphens and underscores read as spaces, the first letter capital, so `foreman-ui` is "Foreman ui". Only how it shows: the repo's id, its folder name, the URL and every other place a repo is named (branch lines, reviews) keep `foreman-ui`. Its id is the name's tooltip;
  - its path (Mono 12px `--tx4`);
  - its main branch (Mono) and `· N branches` (12.5px `--tx3`); or, for a repo with a problem, the problem in `--err` (a missing folder, or one that is another repo: `Locate it where it went, or remove it from the project`);
  - when the repo no longer has its main branch, in `--err`: "No branch <main>: pick its main branch, or agents can't start." (the CEO, 2026-10-21);
  - a **`⋯` menu** (above the card's link): **`Main branch…`** (**`Locate…`** instead for a repo with a problem), **`Rename…`** and **`Remove…`**.
- **The project repo's card**: "Project repo", "project.toml and the wiki", its branch count; no menu.
- With no code repo, above the cards in `--err`: `No code repo yet: agents need at least one to work on.` Then **`Add a repo`**. The routes and their refusals are in [Add a project → Code repos](./add-project.md#code-repos); a refusal is a toast "Repos not changed" with the reason.
  - **Add a repo** opens a modal "Add a repo to <project>" with the [repo picking form](./add-project.md#picking-repos) (Existing repo or New empty repo); it closes once one is added.
  - **Main branch…** opens a modal "Main branch of <name>": "Its agents start from it, and its merges go to it.", and a select of its branches; a pick saves it (`PUT /api/projects/{id}/repos/{repo}/main`, allowed while a team is deployed) and closes it.
  - **Locate…** opens a modal "Locate <repo>": "Pick its folder now. It must be the same repo: Foreman checks they share a commit.", and the folder browser; a pick saves it.
  - **Rename…** opens a modal "Rename <repo>" with the name in a field and **`Rename`**.
  - **Remove…** asks first: "Remove <repo> from <project>?", "Its folder and its branches stay where they are; agents' worktrees of it go. Add it again any time."
  - While a team is deployed on the project, **Rename…** and **Remove…** are disabled, and the menu ends with "Stop the team on this project first to rename or remove it."; the main branch and locating aren't.

## A repo's page

`/projects/<project>/repos/<repo>?branch=<branch>&path=<file>&history=1`, laid out as GitHub does (the CEO, 2026-11-02): a `←` back to the list, the repo's name as words (600 18px; "Project repo" for the project repo) and its id (`<repo>/`, Mono 12.5px `--tx4`); a bar with the branch and its latest commit; then the file tree beside the file open, or the history. The page fills the window's height (at least 420px), and the tree, the file and the history each scroll on their own. The section stays selected in the sidebar.

- **In the URL**, each a step Back returns from (the CEO, 2026-11-02; choosing a branch replaced the entry until then): `branch`, the branch read (`/projects/ledger/repos/api?branch=wt/tove/LED-44`), so a link opens it: an agent's worktree or a branch in flight on [Deployment](./project-deployment.md), a review's branch ([Inbox](./reviews.md)); `path`, the file open; `history`, the history instead of the files. A branch the repo doesn't have falls back to its first branch (`main`, when it has one); a repo the project doesn't have says `<project> has no repo <repo>.` Another branch keeps the file open (a file it lacks shows git's error) and starts its tree folded.
- **The branch** is a button (its name in Mono, `▾`) opening a list to pick from: a box `Find a branch…` (any part of the name, any case; Enter picks the first shown), then the branches, scrolling: `main` first, then the most recently committed first (the CEO, 2026-10-17; same-second ties by name). A branch: its name (Mono 12.5px), and under it `default branch` for `main` (the branch merges go into), whose it is (`wt/<agent>/…` that agent's, `lead/<team>/…` that team's lead's), `N ahead of <main>` when it has commits `main` hasn't, and `uncommitted changes` (`--accTx`) when the worktree it is checked out in has changes not committed yet. The branch shown is on `--accSoft` and in view when the list opens. `No branch has that name.` when none matches.
- **The latest commit** (a bar beside the branch, `--panel`, 48px high, as is the branch button): its author, its subject, its short hash and its time (hidden under 900px); `No commits yet.` for none; on its right **`N commits`** (on `--raised`). The whole bar is one button (the CEO, 2026-11-02): the history takes the page; there the bar's border is `--accLine` and it reads **`← Files`**, back to the tree and the file (the CEO, 2026-11-02: the history was hidden behind a toggle).
- **The tree** (180–260px): the branch's files as committed, folders first (each with its chevron, opening in place, read when opened), then files; a symlink says `link`, a submodule `submodule` (not opened). A click on a file opens it beside the tree (on `--accSoft` while open). The folders above the file open are open, so a link to a file far down shows where it is; the CEO's own folding stays. An empty repo: `Nothing committed here yet.`
- **The file**: its path (Mono 12.5px) and size, staying in view while it scrolls; its text in monospace with line numbers (not selected with the text). With no file picked, the root's README (`README`, `README.md` and the like, any case), else its `index.md`, else `Pick a file on the left to read it.`
  - **A Markdown file** (`.md` or `.markdown`, as the server decides) is rendered, with **Rendered / Source**. Its [frontmatter](./wiki-reader.md#frontmatter) is apart, above it, as in the wiki reader, and a `title` field is its heading; Source shows the file whole. Rendered inert: raw HTML stays off, a web link opens in a new tab, a relative link is text, and an image shows its text, never loaded (agents write these files).
  - A binary file (a NUL byte) or one over 1 MB says so instead.
- **The history**: the branch's commits, or with **All branches** (segmented with the branch's name above them) every branch's, children before parents (`git log --topo-order`, `--branches` for all), 50 at a time with **Show more**, drawn as a **graph** (the CEO, 2026-11-01): each commit a dot on its lane, lines to its parents, as `git log --graph` lays them out (a commit takes the lane waiting for it, its first parent continues it, another parent takes a free lane; lanes never shift sideways; lane colors `--acc`, `--info`, `--ok`, `--tx3` in turn), rows 52px high. Each row: the short hash, the subject (`merge` beside a merge) with, as a second line in `--accTx` Mono, the branches pointing at that commit (the CEO, 2026-10-17; never `HEAD` or a tag), the author, the time (local, with the date when not today).
- **As committed:** files and history are what the branch has committed, never a worktree's files; the `uncommitted changes` flag is the only sign of work in progress. It refreshes when an agent commits, switches or merges (`team.action`), on `pipeline.changed`, and when projects or their repos appear, go or change (`projects.changed`).
- Diffs stay with the merge requests ([Project pipeline](./project-pipeline.md)).

## Data

`GET /api/projects/:id/repos` lists the repos and branches, each branch with `agent`: whose worktree has it checked out (the agent whose sandbox, `<workspace>/agents/<agent>/`, holds that worktree; `""` for the CEO's checkout or none); `…/repos/:repo/tree?ref=&path=`, `…/file?ref=&path=` (a Markdown file with its frontmatter apart) and `…/log?ref=&skip=` (or `…/log?all=1`, every branch's, each commit with its `parents`; each page with the `total` held) read at a ref ([API](../architecture/api.md)). `:repo` is one of the project's code repos or `project`; anything else is `404`. A ref must name a commit and never starts with `-`; a path is relative and can't climb out with `..`; git then resolves it inside that commit's tree, so nothing outside the repo is reachable. The reads are the Git block's (`Resolve`, `Tree`, `File`, `Log`, `Count`, `Worktrees`), composed by the repos block (`internal/repos`).
