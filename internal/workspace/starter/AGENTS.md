# How this project works

This is the project repo: `project.toml` (the project's config, owned by the CEO) and `wiki/`, the team's knowledge base and task board. Every agent of the project reads the same live files.

**Only the team lead writes the wiki**: edit the files, run `wiki --root project/wiki check`, then `commit()`. Everyone else adds through `contribute(kind, title, body)` (`finding`, `proposal` or `note`): Foreman files it in `wiki/inbox/` and tells the lead, who turns it into an entry or a task, or drops it. The CEO may tick a checkbox or move a card from Foreman at any time: re-read an entry before editing it.

## The wiki

Plain Markdown files with YAML frontmatter, plus a `wiki.toml`, operated with the `wiki` command. From your sandbox, always pass `--root project/wiki`:

```sh
wiki --root project/wiki status                       # what's there, and what's broken
wiki --root project/wiki search "settlement"          # find before you write
wiki --root project/wiki read /product/settlement.md  # an entry, frontmatter stripped
wiki --root project/wiki list --where type=task --where status!=done   # open work
wiki --root project/wiki backlinks /product/settlement.md              # what points here
wiki --root project/wiki move /a.md /product/a.md     # move, rewriting every link to it
wiki --root project/wiki check                        # before every commit()
```

Rules that keep it findable:

- **Every entry has a `type`**: `spec`, `task`, `epic` or `note` (`wiki.toml` allows no other). `index.md` files carry none.
- **One thing per entry**, in one folder, with a lowercase hyphenated file name (`settlement-rules.md`). Similar but distinct things get their own entries, linked to each other.
- **Link what relates**, with relative Markdown links (`[Settlement](../product/settlement.md)`). An entry nothing links to is lost: link it from its folder's `index.md`.
- **Never move a file by hand**: `wiki move` rewrites its links. **Search first**: update the entry that exists rather than write a second one.
- **Records you'd count or sum** (measurements, invoices) go in one entry as a Markdown table, not one entry each.

## Structure

```
wiki/
├── index.md            # entry point: links to product, backlog, technical debt (and what you add)
├── technical-debt.md   # accepted workarounds and limitations, one page (type: note)
├── product/            # durable: what the project is, as intended (type: spec)
│   └── index.md        # the product map: links every spec
├── backlog/            # ephemeral: the work (type: task)
│   ├── index.md        # the current goal, and the queries that list the work
│   ├── <task>.md       # one file per task
│   ├── epics/          # groups of tasks, in order (type: epic)
│   └── archive/        # retired work worth keeping
└── inbox/              # contributions waiting for the lead (type: note)
```

**Optional, created when there is something to put in it** (never empty, never as a placeholder): `research/` for what the team found out (investigations, comparisons of options, measurements, a spike's outcome; `type: note`), `product/glossary.md` for the project's terms, or another folder the project really needs (`discovery/`, `issues/`…). Link each new folder's `index.md` from `wiki/index.md`.

**Different lifetimes.** `product/` is durable: it describes the project as intended, and each spec says how much of it is real. `backlog/` is ephemeral: tasks come and go. When a task ships, the knowledge it produced goes into the matching spec (or a research note), and the task is retired. Decisions go into the spec they change, with who and when, not into a folder of their own. Don't make folders for progress (`todo/`, `doing/`): that's what `status` is for.

## Specs

```markdown
---
type: spec
status: planned       # planned | partial | built
---

# Settlement

What it is and how it behaves. Link the related specs.

## Open questions

1. **Short name.** The question, the options, a recommendation.
```

- `planned`: nothing of it exists yet; `partial`: some does (the body says which); `built`: the body matches what shipped. Update the spec in the same change as the work.
- Open questions live in the spec they concern, nowhere else. Ask the CEO (`escalate`, or through the lead); write the answer into the spec and remove the question.

## Tasks

Every piece of work is one **task entry** in `backlog/`. The project's boards show them, one lane per `priority`: **Board** the scheduled ones by `status`, **Backlog** the unscheduled ones. The CEO moves cards there too, which rewrites these same fields. **Never keep tasks as a table or a list in a page**: the boards can't see them.

```markdown
---
type: task
title: Export invoices as CSV
status: backlog       # backlog | todo | blocked | in-progress | done
priority: medium      # quick | high | medium | low
assignee: tove        # the agent id of who does it; leave it out while unassigned
epic: invoicing       # the epic's file name in backlog/epics/
tags: [feature]       # one of: feature | bug | chore | test | decision | spike
---

What and why, and what "done" means. Link the specs it touches.

- [ ] step
- [ ] step
```

- **`status`**: `backlog` (captured, not scheduled), `todo` (scheduled: ready to start), `blocked` (it can't move: say on what in its body, on a line of its own: `Blocked by [Title](./other-task.md)`, or the CEO's answer, a teammate's reply; remove the line when it moves again), `in-progress` (its assignee is on it, including while it is reviewed), `done` (merged or accepted). These five values, no others: a board shows any other value as a column of its own. **Only `todo` tasks are picked up**: the lead (or the CEO, on the Backlog board) schedules a task by moving it from `backlog` to `todo`.
- **`priority`**: `quick` (no uncertainty: it can be done right now, without issue), then `high`, `medium`, `low`.
- **`title`** is what the board shows; without it, the board shows the file name. **`assignee`** is an agent id, so the board shows who has it.
- **Steps** are `- [ ]` checkboxes in the body; the CEO can tick them too.
- **Tags**: `decision` is a question the CEO must answer before work can rely on it; `spike` is throwaway exploration whose outcome goes into a spec.
- **The lead keeps the boards true**: creates the tasks, assigns them, and updates `status` as work moves (a teammate's reply or handoff is the signal). A teammate who needs a task changed tells the lead, or `contribute()`s a proposal.

## Epics

Work is grouped in **epics**, done in order; each delivers something usable on its own.

```markdown
---
type: epic
status: todo          # todo | in-progress | done
order: 1              # the order epics are done in
---

# Invoicing

The goal, what is usable when it's done, its exit criteria.
```

An epic lives in `backlog/epics/<slug>.md`; its tasks name it in `epic: <slug>`. Write tasks for the current epic and the next; later epics say in their body what they will contain.

## Lifecycle

1. **Capture**: a new task in `backlog/` with `status: backlog` and a `priority`. Contributions in `inbox/` become tasks, specs, research notes, or nothing; then delete them.
2. **Schedule**: the lead moves it to `todo` when it's next.
3. **Work**: `in-progress`, ticking its steps, through review; `blocked`, with its `Blocked by` line, while it can't move.
4. **Finish**: update the specs it touched (and their `status`), set `done`, then retire the task: delete it (git keeps it; remove links to it first, `wiki backlinks` finds them), or `wiki move` it to `backlog/archive/` when a browsable record is worth keeping.

**Technical debt** (every workaround or `TODO`) is an item in `technical-debt.md`: what, why, how to fix. When one is scheduled, a task links to it; when fixed, the item goes.

## Finding things

- **What's next?** `wiki --root project/wiki list --where type=task --where status=todo --where priority=quick`, then `priority=high`
- **Unscheduled:** `--where type=task --where status=backlog`
- **In progress, blocked:** `--where type=task --where status=in-progress` (or `status=blocked`)
- **An epic's work:** `--where epic=invoicing --where status!=done`
- **What the CEO must decide:** `--where type=task --where tags=decision --where status!=done`
- **How far is the build:** `wiki --root project/wiki property status --counts --where type=spec`
- **How does X work:** start at `product/index.md`, or `wiki --root project/wiki search "x"`
- **Housekeeping** (the lead): `wiki --root project/wiki check`, `orphans` (link them), `unresolved` (links to entries not written yet), done tasks not yet retired, stale and blocked ones.
