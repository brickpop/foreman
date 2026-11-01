---
type: task
status: done
priority: high
tags: [feature, ui, bug]
epic: workflow
---

# Repos as GitHub lays them out, frontmatter as wikiview shows it, big reviews stay quick

**Why:** the CEO's feedback after [Projects: order, sidebar toggle, Files…](../project-polish.md) (2026-11-02): a review of a commit with a great many files was sluggish; the repo rows were hard to click and named like paths; a repo's branches, files and history were all in one cramped browser, the history hidden behind a toggle.

## Decided (the CEO, 2026-11-02)

- **Big reviews**: no virtualization (it would cost the browser's find, the sticky file headers and the comment boxes); the lighter fix instead: large files (over 500 changed lines) and lock files start folded, as on GitHub; a folded file's lines aren't built; open files are drawn only near the view; a click redraws only its file ([Reviews → A review's page](../../product/features/reviews.md#a-reviews-page)).
- **Repos as cards**, each opening its repo wherever clicked, the name as words (`foreman-ui` reads "Foreman ui"), the changes in a `⋯` menu ([Repos → The list](../../product/features/repos.md#the-list)).
- **A repo's page as GitHub's**: the branch a dropdown with a find box, the latest commit in a bar with `N commits`, the file tree beside the file, the README (else `index.md`) until a file is picked; the history takes the page ([Repos → A repo's page](../../product/features/repos.md#a-repos-page)).
- **Frontmatter as wikiview shows it**, everywhere a Markdown file shows (reader, card sheet, repo files): folded to a line, open to a grid, tags with colors, references linking their entries ([Wiki reader → Frontmatter](../../product/features/wiki-reader.md#frontmatter)).

## Done

- [x] Review: memoized file boxes, the diff parsed once, `content-visibility`, folded-at-first files (`foldedAtFirst` in `ui/src/app/diff.ts`)
- [x] `GET …/file` gives a Markdown file's frontmatter and body; `GET …/log` the total; an entry's frontmatter as ordered fields with references resolved
- [x] Repos cards and the `⋯` menu; the repo page (branch picker, latest commit, tree, README, history)
- [x] The `Frontmatter` component, replacing the chips
- [x] Specs: Repos, Reviews, Wiki reader, Board, Wiki engine, API, Design system; technical debt (the copied reference rule and panel)

## Left out

- Hovering the folded frontmatter floats the grid in wikiview; here a click opens it.
- A commit doesn't open its diff yet (still open on [Projects: order, sidebar toggle, Files…](../project-polish.md)).
