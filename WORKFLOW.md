# How the knowledge base works

This is the **workflow** layer for the `wiki/` bundle: the conventions this project follows on top of what [AGENTS.md](./AGENTS.md) and the `wiki` tool define. Edit it when the way we work changes.

All commands run from the project root with `--root wiki` (or, without it, from inside `wiki/`: `wiki` finds its bundle by walking up to a `wiki.toml`). All entry paths below (`/backlog/x.md`) are relative to `wiki/`.

**The bundle is git-managed.** Commits are its history and its undo: retiring a task by deleting it loses nothing, and every `wiki move` or edit is reversible.

## Structure

```
wiki/
├── index.md             # entry point: links to product, backlog, technical debt
├── technical-debt.md    # accepted limitations and workarounds (type: note)
├── product/             # durable knowledge: what Foreman is (type: spec)
│   ├── index.md         # the product map
│   ├── vision.md        # summary, goals, non-goals, roles
│   ├── glossary.md
│   ├── use-cases.md     # end-to-end scenarios (the acceptance tests)
│   ├── models/          # config files and runtime state
│   ├── features/        # user-facing behaviour, one screen or flow per entry
│   ├── architecture/    # components, protocols, stack
│   └── design/          # design system, the UI prototype
└── backlog/             # ephemeral: the kanban
    ├── index.md         # entry point: the goal and the queries that list the cards
    ├── <card>.md        # every live card (type: task), read with `wiki list` (ignore_orphans'd)
    ├── epics/           # the epics: groups of cards, in order (type: epic)
    └── archive/         # retired work worth keeping (ignore_orphans'd)
```

**Two halves with different lifetimes:**
- **`product/` is durable.** It describes Foreman *as intended*, and each entry says how much of that is real through its `status` (below). When code ships, the spec is updated in the same change so that it describes the shipped behaviour exactly; a `built` spec is as-built documentation.
- **`backlog/` is ephemeral.** Cards come and go. When a card ships, whatever lasting knowledge it produced is folded into the matching `product/` entry, and the card is deleted.

## Entry types

| `type` | Lives in | What |
|---|---|---|
| `spec` | `product/` | One concept of the product: a model, a feature, an architectural piece, a design element |
| `task` | `backlog/` | One coherent piece of work (including a decision to take) |
| `epic` | `backlog/epics/` | A group of cards that delivers something usable on its own, in a fixed order |
| `note` | anywhere | Everything else (e.g. `technical-debt.md`) |

`wiki/wiki.toml` enforces this vocabulary: `wiki check` errors on any other type.

**Tags** (cross-cutting, fixed vocabulary):
- Specs: exactly one kind, `model` / `feature` / `architecture` / `design`, plus topics: `home`, `config`, `teams`, `agents`, `sessions`, `requests`, `pipeline`, `team-tools`, `git`, `wiki`, `ui`, `api`, `security`, `runtime`.
- Tasks: exactly one work-kind, `feature` / `bug` / `chore` / `test` / `decision` / `spike`.
  - `decision`: a question the user must answer before the affected specs can be relied on.
  - `spike`: throwaway exploration to de-risk a design (a prototype, a measurement); its outcome is folded into a spec.

## Specs

```markdown
---
type: spec
status: planned           # planned | partial | built
tags: [feature, sessions]
---

# Title

What it is and how it behaves. Link the related specs generously.

## Open questions

1. **Short name.** The question, the options, and a recommendation if there is one.
```

- **`status`**: `planned` (nothing of it exists in code), `partial` (some of it does; the body says which parts), `built` (the body matches the code). Anything described in a `planned` or `partial` spec that is not built is a target, not a fact.
- **Open questions** live in the spec they concern, in a final `## Open questions` section, and nowhere else. A `decision` card points to them (`[Sessions](../../product/features/sessions.md#open-questions)`) rather than copying them. When the user answers, write the answer into the spec body, remove the question, and update or retire the card.
- **Source:** the original PRD (v1.0, 2026-10-12) and the UI prototype were split into these entries; the prototype is kept as a reference in [`product/design/prototype/`](./wiki/product/design/prototype/). There is no separate PRD file: `product/` replaced it.

## Tasks

```markdown
---
type: task
status: todo              # todo | in-progress | blocked | done
priority: medium          # high | medium | low
tags: [feature]           # feature | bug | chore | test | decision | spike
epic: terminal-multiplexer # the slug of its epic: the file name in backlog/epics/
---

# Short imperative title

Why this matters, and what "done" means. Link the spec entries it touches,
e.g. [Sessions](../product/features/sessions.md).

- [ ] step
- [ ] step
```

- **Progress** is `status`. **Urgency** is `priority`. **Belonging** is `epic`. There is no hand-maintained board: the backlog is read with `wiki list` and filters (see *Querying the backlog*), or browsed on the boards declared in `wiki.toml` (see *Boards*), so nothing has to be kept in sync.
- **Steps** are `- [ ]` checklist items inside the card (`wiki --root wiki checkboxes /backlog/x.md`).
- **Dependencies** are body links: `Blocked by [Title](./other-task.md)`. A real graph edge, so `wiki backlinks` on the prerequisite shows what it unblocks and `wiki move` keeps it intact. A card waiting on another also gets `status: blocked`.
- **Detail pages:** a card that needs notes keeps them in its body. If it outgrows that, add a `that-task/` folder beside `that-task.md` (never a `that-task/index.md`).
- **Epic:** every card belongs to one epic, named by its `epic:` field (the epic's slug: its file name in `backlog/epics/`). A field rather than a link, so the Backlog board can lay the cards out in one lane per epic and `--where epic=<slug>` lists exactly an epic's cards. The epic itself links its cards' specs and journey steps, not each card.
- Not tracked: estimates, assignees, cycles, milestones. Add them here first if they ever become necessary.

## Epics

Work is organised in **epics**, done in order. Each epic delivers something usable on its own and has exit criteria; the next one starts when it's done.

```markdown
---
type: epic
status: todo              # todo | in-progress | done
order: 1                  # 0, 1, 2…: the order epics are done in
---

# Terminal multiplexer

The goal, what is usable when it's done, the journey steps it covers, its exit criteria.
```

- An epic lives in `backlog/epics/<slug>.md`. Its cards name it in their `epic:` field (`epic: <slug>`), so `wiki --root wiki list --where epic=<slug>` lists them. A new epic is also added to the Backlog board's `lanes` in `wiki.toml`, in order.
- Two epics may share an `order` when they run side by side.
- Cards are written for the current epic and the next one; later epics get theirs when we get there. An epic's body says what it will contain until then.
- An epic is `done` when its exit criteria hold and its cards are retired. Retire it like a card (delete, or move to `archive/`).

## Technical debt

Debt is catalogued in `wiki/technical-debt.md` (what, why, how to fix), not as cards. Every workaround or `TODO` comment gets an item there. When a debt item gets scheduled, create a card that links to it. When it's fixed, remove the item.

## Lifecycle

The folder says only whether a card is live (`backlog/`) or retired (`backlog/archive/`). Progress lives in `status`, urgency in `priority`.

1. **Capture.** A new item is a `type: task` file directly in `backlog/`, with `status: todo` and a `priority`.
2. **Work.** Set `status: in-progress`, advance it in place (`blocked` when waiting), tick its steps. Change `priority` to reschedule.
3. **Finish.** Update the affected `product/` entries so they describe the shipped behaviour (and their `status`). Set `status: done`. Retire the card (below).

**Don't turn progress or scheduling into folders** (`todo/`, `doing/`, `now/`, …). Both change often, so encoding them as folders means a file move on every change. The only folder move is `backlog/` → deleted or `archive/`.

## The backlog entry point

`wiki/backlog/index.md` is the entry point, not a board: the **goal** of the current phase and the queries below. It does not list cards. Cards are kept out of `wiki orphans` by `ignore_orphans = ["backlog/**"]`, since nothing has to link to them.

## Boards

`wiki.toml` declares two kanban boards over the same entries, for WikiShell or Foreman itself to show:

- **Epics**: the epics by `status` (To do → In progress → Done): the roadmap at a glance.
- **Backlog**: the cards by `status`, in one lane per epic, in the epics' order. Priority stays on each card.

The boards are views: the entries own their `status`, `priority` and `epic`, and moving a card on a board rewrites that field. Specs are not on a board: they are the definition, not work (their build status is a query, see below).

## Querying the backlog

- **Which epic are we in?** `wiki --root wiki list --where type=epic --where status=in-progress`; all of them, in order: `wiki --root wiki list --where type=epic --format json` (sort by `order`).
- **What's in an epic?** `wiki --root wiki list --where epic=terminal-multiplexer`; still open: add `--where status!=done`.
- **What's next?** The current epic's cards with `status: todo`, high priority first (`wiki --root wiki list --where epic=<slug> --where status=todo --where priority=high`); `wiki --root wiki list --where type=task --where status=todo --where priority=high --prefix backlog/` across all epics. Read the cards and use judgment: blockers and decisions first.
- **What's in progress?** `wiki --root wiki list --where type=task --where status=in-progress`.
- **Everything live:** `wiki --root wiki list --where type=task --where status!=done --prefix backlog/` (archived cards are `done`).
- **What does the user need to decide?** `wiki --root wiki list --where type=task --where tags=decision --where status!=done`.
- **What's blocked?** `wiki --root wiki list --where type=task --where status=blocked`.
- **How far is the build?** `wiki --root wiki property status --counts --where type=spec`, or `wiki --root wiki list --where type=spec --where status=planned`.
- **How does X work?** Start at `wiki/product/index.md`, or `wiki --root wiki search "x"`.
- **What touches X?** `wiki --root wiki backlinks /product/features/x.md` (the cards and specs linking to it).
- **Tally by status:** `wiki --root wiki property status --counts --where type=task`.

Anything beyond a filter (roll-ups, reports) is a small script over `wiki --root wiki list --format json`.

## Retiring done work

- **Delete it** (the default): delete the file. Git keeps the record. Lasting knowledge belongs in `product/`. Remove any `Blocked by` links to it first (`wiki --root wiki backlinks /backlog/x.md`).
- **Archive it** (only when a browsable record is worth keeping): `wiki --root wiki move` it to `/backlog/archive/` (links to it are rewritten).

## Grooming

- Run `wiki --root wiki check` and fix what it flags.
- `wiki --root wiki list --where type=task --where status=done --prefix backlog/`: finished cards not yet retired (archived ones show up too; skip `backlog/archive/`).
- Review stale cards (`wiki --root wiki list --where type=task --sort=timestamp --reverse`) and blocked ones: unblock, drop, or reprioritise.
- Review `wiki/technical-debt.md` regularly: schedule, accept, or fix.
- Review open questions (`wiki --root wiki search "Open questions" --exact`): every one should have a `decision` card pointing at it.
- `wiki --root wiki orphans`: a hit is a spec that nothing links to. Link it from `product/index.md` or a related spec.
- `wiki --root wiki unresolved`: links to entries not yet written.
