---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# The wiki at the project repo's root

**Why:** the CEO, 2026-11-04: `project/wiki/backlog` should read `backlog`. The project repo holds little else (`AGENTS.md`, `project.toml`), so it can be the wiki bundle itself: `wiki.toml`, `index.md`, `product/`, `backlog/` at its root. Second of two changes, after [One Foreman folder](./foreman-folder.md) (split on 2026-11-05); independent of it in code, the two meet only in the agents' prompt and the CEO's move script (on that card).

```
projects/<project>/   wiki.toml, index.md, product/, backlog/, inbox/, AGENTS.md, project.toml
```

**Steps:**

- [ ] the starter scaffold at the root, with `ignore = ["AGENTS.md", "README.md"]` so the operating docs aren't entries (and `project.toml`, not Markdown, isn't indexed anyway)
- [ ] where the wiki is read (the reader, the board, `wiki` calls), the contribute inbox (`inbox/`), the prompt's line naming the wiki's place
- [ ] a project repo still keeping it in `wiki/` (a clone, an old project) gets a notice saying to move it to the root, not "Create it"
- [ ] specs: wiki-reader, board, session-prompt, project-config, add-project, first-run
- [ ] tests: a project repo made with the wiki at its root, a `wiki/` one getting the notice, the board and reader reading the root
