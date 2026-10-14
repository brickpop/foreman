---
type: task
status: todo
priority: medium
tags: [feature]
---

# Default project wiki conventions

Epic: [Terminal multiplexer](./epics/terminal-multiplexer.md)

**Why:** agents keep a project's knowledge and tasks in its wiki, and the board reads task cards from it ([Board](../product/features/board.md), [Wiki engine](../product/architecture/wiki-engine.md)). For the board to show what agents write, every project wiki needs the same conventions: what a task card is (`type: task`, `status`, `assignee`, `priority`…), where cards live, and how agents use the `wiki` command. Foreman's own repo already works this way (`WORKFLOW.md` and `AGENTS.md` at its root).

**Done when:** Foreman ships a default `WORKFLOW.md` and `AGENTS.md` for project wikis, Add a project scaffolds them with the wiki (`wiki.toml`, `index.md`, `backlog/`) when the project has none, the default board in `wiki.toml` matches the card format, and every session prompt points agents at them ([Session prompt](../product/architecture/session-prompt.md)).

- [ ] write the default `WORKFLOW.md` and `AGENTS.md` (task card format the board reads; statuses; assignee = agent id)
- [ ] scaffold them in [Add a project](../product/features/add-project.md); never overwrite existing files
- [ ] the default board declaration in `wiki.toml`
- [ ] reference them from the session prompt template
- [ ] specs: Add a project and Session prompt describe the scaffold and the reference
