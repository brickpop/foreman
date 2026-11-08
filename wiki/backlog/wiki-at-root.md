---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# The wiki at the HQ's root

**Why:** the CEO, 2026-11-04: `project/wiki/backlog` should read `backlog`. A project's HQ (was "the project repo", renamed by [One Foreman folder](./foreman-folder.md)) holds little else (`AGENTS.md`, `project.toml`), so it can be the wiki bundle itself: `wiki.toml`, `index.md`, `product/`, `backlog/` at its root. Foreman's HQ follows the same shape (the CEO, 2026-11-08): a wiki at its root, its `teams/` and `instructions.md` beside it, the Director's board in `outcomes/` ([The Director's board](./director-board.md)). Second of two changes, after [One Foreman folder](./foreman-folder.md) (split on 2026-11-05); independent of it in code, the two meet only in the agents' prompt and the CEO's move script (on that card).

```
projects/<project>/hq/   wiki.toml, index.md, product/, backlog/, inbox/, AGENTS.md, project.toml
hq/                      wiki.toml, index.md, outcomes/, teams/, instructions.md
```

**Steps:**

- [ ] the project HQ's starter scaffold at the root, with `ignore = ["AGENTS.md", "README.md"]` so the operating docs aren't entries (and `project.toml`, not Markdown, isn't indexed anyway)
- [ ] Foreman's HQ scaffold: `wiki.toml` (`ignore = ["instructions.md"]`: the CEO's instructions are not an entry), `index.md` linking the board; the `outcome` type and the board come with [The Director's board](./director-board.md)
- [ ] where a project's wiki is read (the reader, the board, `wiki` calls), the contribute inbox (`inbox/`), the prompt's line naming the wiki's place
- [ ] an HQ still keeping it in `wiki/` (a clone, an old project) gets a notice saying to move it to the root, not "Create it"
- [ ] specs: wiki-reader, board, session-prompt, project-config, add-project, first-run, home
- [ ] tests: an HQ made with the wiki at its root, a `wiki/` one getting the notice, the board and reader reading the root
