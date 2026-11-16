---
type: task
status: done
priority: high
tags: [feature]
epic: install-first-run
---

# The wiki at the HQ's root

**Why:** the CEO, 2026-11-04: `project/wiki/backlog` should read `backlog`. A project's HQ (was "the project repo", renamed by [One Foreman folder](../foreman-folder.md)) holds little else (`AGENTS.md`, `project.toml`), so it can be the wiki bundle itself: `wiki.toml`, `index.md`, `product/`, `backlog/` at its root. Foreman's HQ follows the same shape (the CEO, 2026-11-08): a wiki at its root, its `teams/` and `instructions.md` beside it, the Director's board in `outcomes/` ([The Director's board](../director-board.md)). Second of two changes, after [One Foreman folder](../foreman-folder.md) (split on 2026-11-05); independent of it in code, the two meet only in the agents' prompt and the CEO's move script (on that card).

```
projects/<project>/hq/   wiki.toml, index.md, product/, backlog/, inbox/, AGENTS.md, project.toml
hq/                      wiki.toml, index.md, outcomes/, teams/, instructions.md
```

**Decided while building** (the CEO, 2026-11-16): every agent's environment names its project's HQ as **`PROJECT_HQ`** (not `FOREMAN_HQ`, which reads as Foreman's HQ), so the starter's AGENTS.md says `wiki --root "$PROJECT_HQ"` on any machine.

**Steps:**

- [x] the project HQ's starter scaffold at the root, with `ignore = ["AGENTS.md", "README.md"]`; its AGENTS.md and backlog index rewritten for the root, `$PROJECT_HQ` and `commit()` naming `hq` (it still said `--root project/wiki` and `project`, stale since One Foreman folder)
- [x] `PROJECT_HQ` in every agent's environment (both engines)
- [x] Foreman's HQ scaffold: `wiki.toml` (`ignore = ["instructions.md"]`), `index.md`, `.wiki/` ignored; the `outcome` type and the board come with [The Director's board](../director-board.md)
- [x] where a project's wiki is read (`Home.WikiDir`: the HQ), the contribute inbox (`inbox/`), the prompt's lines (knowledge base, goal, teammates)
- [x] an HQ still keeping it in `wiki/` gets a notice with the `git mv` to run, not "Create it"; the scaffold refuses (400) while it is there
- [x] specs: wiki-reader, board, session-prompt, project-config, add-project, home, api, glossary, journey, use-cases, wiki-engine, persistence, protocol, team-tools, building-blocks, sandboxes-and-git
- [x] tests: an HQ made with the wiki at its root passing `wiki check` (a project's and Foreman's), a `wiki/` one told to move and not scaffolded over, the board reading the starter's root `wiki.toml`, contributions in `inbox/`, `PROJECT_HQ` in the lead's and a member's environment
