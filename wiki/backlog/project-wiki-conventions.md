---
type: task
status: todo
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Default project conventions: AGENTS.md

**Why:** agents keep a project's knowledge and tasks in its wiki, and the board reads task cards from it ([Board](../product/features/board.md), [Wiki engine](../product/architecture/wiki-engine.md)). Without shared conventions they invent their own: in the dev workspace's `DevProject`, the team kept its tasks as a Markdown table in `gravity-tasks.md`, so the Backlog board shows no cards (seen 2026-10-17). Every project needs the same conventions: what a task card is (`type: task`, `status`, `assignee`, `priority`…), where cards live, and how agents use the `wiki` command.

**Decided (the CEO, 2026-10-17):**
- **One `AGENTS.md`** with everything (operating the wiki, the task card format the board reads, the statuses), not Foreman's own AGENTS.md + WORKFLOW.md split.
- At **`project/AGENTS.md`**, the project repo's root beside `project.toml`, outside the wiki bundle (so `wiki check` doesn't want a type on it).
- It joins the **scaffold** ([Add a project → Effect](../product/features/add-project.md#effect)): written by Add a project, and offered by the [No wiki yet](../product/features/wiki-reader.md#no-wiki-yet) placeholder to projects that lack it, never overwriting one.
- Order: the placeholder and the shared scaffold first (done 2026-10-17), then this file, drafted with the CEO.

**Done when:** Foreman ships a default `AGENTS.md`, the scaffold writes it, the default board in `wiki.toml` matches its card format, and every session prompt points agents at it ([Session prompt](../product/architecture/session-prompt.md)).

- [ ] draft `AGENTS.md` with the CEO (task card format the board reads; statuses; assignee = agent id; how to use `wiki`)
- [ ] add it to the scaffold (`internal/workspace/projects.go`); the placeholder then lists it for existing projects. Decide whether a project missing only `AGENTS.md` shows the placeholder, or a lighter notice: the wiki itself works without it
- [ ] the default board declaration in `wiki.toml` matches the card format
- [ ] reference it from the session prompt template
- [ ] specs: Add a project and Session prompt describe it
