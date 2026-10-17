---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: terminal-multiplexer
---

# Default project conventions: AGENTS.md and the wiki structure

**Why:** agents keep a project's knowledge and tasks in its wiki, and the board reads task cards from it ([Board](../product/features/board.md), [Wiki engine](../product/architecture/wiki-engine.md)). Without shared conventions they invent their own: in the dev workspace's `DevProject`, the team kept its tasks as a Markdown table in `gravity-tasks.md`, so the Backlog board shows no cards (seen 2026-10-17).

**Decided (the CEO, 2026-10-17):**
- The default is **modelled on Foreman's own AGENTS.md + WORKFLOW.md** and its wiki structure (`product/` specs with a status, `backlog/` tasks grouped in epics, `technical-debt.md` as one page, boards for epics and for tasks, here laned by priority; decisions go into specs, not a folder; optional parts such as `research/` or a glossary described in AGENTS.md but created only when needed), in **one `AGENTS.md`**.
- At **`project/AGENTS.md`**, the project repo's root, outside the wiki bundle.
- Statuses `backlog` (unscheduled), `todo` (scheduled), `blocked`, `in-progress`, `review`, `done`, shown on two task boards (Board: scheduled; Backlog: unscheduled) rather than six columns; priorities `quick` (no uncertainty, doable right now), `high`, `medium`, `low`; the Backlog board's lanes are the priorities; **the lead alone** keeps the board true.
- It is part of the **scaffold** ([Add a project → Effect](../product/features/add-project.md#effect)), the starter folder `internal/workspace/starter/`: written by Add a project, offered to projects lacking it by a **notice** above the wiki (the full [No wiki yet](../product/features/wiki-reader.md#no-wiki-yet) placeholder is only for what the wiki can't be read without).
- The session prompt points every agent at it, one line under "Knowledge base".

**Done when:** a new project gets the structure and AGENTS.md, an existing one is offered what it lacks, and a team deployed on a new project keeps its tasks as entries the board shows.

- [x] the starter folder: `AGENTS.md`, `wiki/wiki.toml` (types, boards with columns and lanes), `wiki/index.md`, `product/index.md`, `backlog/index.md`, `technical-debt.md`; a new project's wiki passes `wiki check` (2026-10-17)
- [x] the scaffold reads it; missing files carry why and whether required; the notice in Overview and Board
- [x] the session prompt line; specs: Add a project, Wiki reader, Session prompt
- [ ] the CEO reviews `internal/workspace/starter/AGENTS.md`
- [ ] seen live: a team on a new project writes its tasks as entries, and they show on the board
