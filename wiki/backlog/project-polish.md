---
type: task
status: in-progress
priority: high
tags: [feature, ui, bug]
epic: workflow
---

# Projects: order, sidebar toggle, Files, a decided review's diff, Repos as a list, the commit graph

**Why:** the CEO's feedback after [The project is the workplace](./project-workplace.md) (2026-11-01).

## Decided (the CEO, 2026-11-01)

- **Projects latest first**, by **last activity** (a team message or action, a request, a merge request, a deploy; else its creation), in the sidebar and on the Dashboard. The order is set when the page loads and doesn't reshuffle while the CEO works; a project added meanwhile comes first.
- **Sidebar**: a click on the open project's name collapses or expands its sections, staying on the page; on another project it opens it, expanded. One project expanded at a time.
- **Wiki**: the file tree always shown, the `Files` toggle gone; under 760px the toggle stays, the tree closed by default.
- **A decided review keeps its diff** (bug: a merged review's page was empty). When a repo of a review stops being open (merged, dismissed, withdrawn), Foreman records the branch's head and the commit it parted from, and the page shows that diff read-only. If git has since dropped the commits (a squash-merged branch deleted and collected), the page says so.
- **Repos becomes the list of code repos**, with what Settings → Code repos held (add, locate, main branch, remove, problems). A repo opens its own page, `/projects/<project>/repos/<repo>`: its branches and the code browser. Settings keeps the workspace, profile, `project.toml`, forge, archive and remove.
- **The commit graph**, only if it stays small: the repo page's history drawn as a graph of every branch (lanes, branch labels), a commit opening its diff. Otherwise kept simple and left as a card.

## Steps

- [x] server: `GET /api/activity/latest` (each project's last activity); tests
- [x] server: a merge request's `head` and `base` recorded when it leaves open; the review diff of a decided repo from them; tests
- [x] UI: projects ordered at load (sidebar, Dashboard); tests
- [x] UI: sidebar toggle on the open project
- [x] UI: Wiki file tree always shown (toggle under 760px)
- [x] UI: a decided review's diff, read-only
- [x] UI: Repos list (from Settings) and the repo page; Settings without code repos
- [x] commit graph: every branch in lanes beside the history. Left out to stay small: a commit opening its diff (needs a commit-diff endpoint), branch labels on the lanes
- [x] specs: Project page, App shell, Dashboard, Wiki reader, Reviews, Review model, Repos, Project settings, API
- [ ] tried by the CEO in the browser
