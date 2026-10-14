# Foreman

A self-hosted server and web UI that lets one person (the CEO) run several teams of AI coding agents across several projects on their own machine. Agents run as real coding-harness processes (Claude Code, Codex CLI, …) in isolated sandboxes, talk to each other through Foreman, and reach `main` only through a pull request that the CEO merges.

The stack is defined in [wiki/product/architecture/tech-stack.md](./wiki/product/architecture/tech-stack.md). Don't restate it here: that entry is the single source of truth.

## Working with the project (CRITICAL)

All project knowledge and work tracking lives in the **`wiki/` bundle**, operated with the `wiki` CLI (see [Operating the knowledge base](#operating-the-knowledge-base-wiki) below):

- **[`wiki/product/`](./wiki/product/index.md)**: the product definition (vision, data models, features, architecture, design system). It's the source of truth for how Foreman behaves. Each spec carries a `status` (`planned` / `partial` / `built`). Keep it in sync with the code in the same change.
- **[`wiki/backlog/`](./wiki/backlog/index.md)**: the task cards, consumed with `wiki list` and filters. It's the reference place when doing work. Adding new cards matters as much as retiring redundant or outdated ones.
- **[`wiki/technical-debt.md`](./wiki/technical-debt.md)**: every workaround or limitation that's added temporarily goes here, with the reason and the planned fix. Review it often.
- [WORKFLOW.md](./WORKFLOW.md) defines the conventions: entry types, tags, spec and task fields, lifecycle and how to query the backlog.

Foreman manages project wikis in this shape (a bundle with a `wiki.toml`), at `<project>/project/wiki/`. This repo doesn't follow that `project/` layout yet: its wiki lives in the code repo.

## Non functional requirements

### Code Quality Rules (CRITICAL)

1. **Simplicity First and above all**
   - Write the simplest solution that solves the problem
   - Aim for "less is more": the small, beautiful tool that solves an entire class of problems
   - Avoid premature optimization
   - Don't get distracted with other non-requirements
   - No abstractions without clear benefit *inside* a component. Between components it's the opposite: every boundary is an interface (see Flexibility and modularity)
   - Code should be obvious at first glance

2. **Only Build What's Needed**
   - Don't implement features "just in case"
   - Don't add plumbing before it's required
   - Iterate based on actual needs

3. **Consistent Patterns**
   - One correct way to do each thing
   - Same problem = same solution everywhere
   - Predictable file locations
   - Consistent naming conventions

4. **No Bloat**
   - Challenge every line of code
   - Remove code no longer used
   - After changing something, see what can be simplified/cleaned up
   - Avoid unnecessary dependencies
   - Keep it lean and maintainable

5. **Documentation via Code**
   - Self-documenting code > comments
   - Clear naming > explanations
   - Only comment the "why", not the "what"

6. **Flexibility and modularity** CRUCIAL
   - The codebase needs to be modular and flexible. **Interfaces, interfaces, interfaces:** every boundary between components is an interface, even with one implementation, so any piece can be swapped at any time.
   - Components need to be easily reusable and swappable: the rules and the list of seams are in [Architecture overview → Modularity](./wiki/product/architecture/overview.md#modularity)

7. **Discoverability**
   - Things should be easy to find out, always following the same pattern
   - No undiscoverable black magic
   - Make sure that the initial setup is very straightforward.
   - If some knowledge can't be made obvious, the README.md explains it in the most straightforward way.

8. **Tests ship with the code**
   - Nothing is shipped until it has meaningful tests: outcomes and edge cases, tests that challenge the code rather than mirror it.
   - Testing for the sake of it is wasted effort.

### Workflow Requirements

**MUST DO:**

0. **When you are doing changes**
    - Ask yourself what side effects you introduce elsewhere: what might break, what becomes obsolete
    - Keep things consistent:
        - One correct way of doing things, only.
        - If you introduce another approach, update every other place still using the old one.
        - Single source of truth (code, data, types)
        - Before adding, ask if the new piece of code may already exist
    - Make sure the card in progress captures enough detail of what's in flight. Compacting a conversation may lose important details otherwise
    - Ensure that new test specs are recorded before completing the card (avoid bloat though), same for the backlog, the affected `wiki/product/` entries and potentially the README
    - Clean up after yourself

1. **Use the knowledge base religiously**
    - Track all work (current, planned, completed) as `type: task` entries (see [WORKFLOW.md](./WORKFLOW.md))
    - Document blockers and decisions
    - Keep it updated and clean: run `wiki --root wiki check` after every batch of edits
    - Add context for future developers, but not bloat

2. **Use `just` and the `justfile` for tasks**
    - Keep commands simple and clear: one stop shop
    - Public recipes have a `# comment` above them so `just help` lists them, and related recipes share a `[group('name')]`
    - The UI uses **Bun only**: `bun install`, `bun run`, `bunx`. Never npm, yarn, pnpm or node. `ui/bunfig.toml` makes every script run on the Bun runtime.

3. **Track technical debt**
    - Every workaround → item in `wiki/technical-debt.md`
    - Every TODO comment → item in `wiki/technical-debt.md`
    - Explain why the workaround exists
    - Plan when/how to fix it

4. **Check for obsolete code**
    - After every change, ask: "What became unused?"
    - Add removal tasks to the backlog
    - Simplify/compact when possible

5. **Maintain consistency**
    - Check existing patterns before writing new code
    - Follow the established conventions
    - Update documentation when patterns change

6. **Ambiguity is not an option**
    - Never let the user continue while there are matters that are not clear or confusing
    - Any topic that is not crystal clear on how to execute must be resolved with the user first
    - Otherwise, you would be adding code that would be obsolete on day 1
    - Report every flaw, vulnerability, inconsistency, attack vector, undefined behaviour and edge case you observe

7. **Garbage free**
    - Messy code is easy to add but very hard to remove
    - Avoid a workaround that fixes an issue now but introduces chronic future headaches.
    - Modular, simple, composable building blocks

8. **Preexisting errors are not OK to ignore**
    - A failing test is telling you something. Fix it, or ask before leaving it.

**MUST NOT DO:**

1. ❌ Add abstractions inside a component without clear need (boundaries between components are always interfaces)
2. ❌ Skip backlog or `wiki/product/` updates
3. ❌ Mixed conventions
4. ❌ Make decisions that weren't defined yet without asking first

## Operating the knowledge base (`wiki`)

The [`wiki/`](./wiki/index.md) folder is an **agentic wiki bundle**: plain Markdown (someone's notes, documents, datasets, and tasks) that you operate with the `wiki` CLI. The goal is knowledge that stays plain and human-readable (portable, greppable, diffable, open in any editor or by any LLM, the user's forever) yet is **structured well enough to answer questions like a database**. Three things make that work, and upholding them is your job:

- **Every entry has one home** (a folder), **one kind** (`type`), and **tags** for what cuts across. So the base is never a loose pile: you can ask for every dataset, everything tagged `2026`, everything under `finance/`.
- **Entries link to what they relate to**, with relative Markdown links (`../finance/income.md`), so they navigate in any tool (GitHub, an editor, Obsidian). Those links form a **graph** (you can see what points at a page), what has no home (orphans), and what's been promised but not yet written (unresolved). A well-kept base can be *navigated* from `index.md` top-down, not grepped blindly.
- **The structure is the value.** The better everything is filed and linked, the more findable it all becomes. Filing precisely, linking generously, and never leaving a dangling reference is the point, not overhead.

`grep` and `ls` can't compute that graph or answer "which notes have no home." `wiki` indexes the folder and does, deterministically. The files are the source (the index `wiki` builds is derived and disposable): **you are the librarian, `wiki` is your database engine.** So for anything structural (finding, moving, checking) call `wiki` rather than guessing or hand-editing paths.

`wiki` runs within the bundle (it walks up to find `wiki.toml`) or from anywhere with `wiki --root <dir>`. **From the project root, always pass `--root wiki`** (e.g. `wiki --root wiki status`): the project root has no `wiki.toml`, so a bare `wiki` finds nothing. All entry paths below and in `WORKFLOW.md` (`/index.md`, `/backlog/x.md`, …) are relative to the bundle root, `wiki/`. This section teaches *how to operate it*; run `wiki <command> -h` for a command's exact flags, and `wiki help` for the full list.

**How *this* base is organized (its types, folders, and the loop you follow) lives in [WORKFLOW.md](./WORKFLOW.md); read it next.**

### The model

Three orthogonal axes classify every entry (keep them separate and the base stays friction-free):

- **Folder** = one stable home, by domain (`finance/`, `tech/infra/`). Moving is `wiki move` (it rewrites the links for you); never hand-move a file.
- **`type`** = what an entry *is* (`note`, `concept`, `dataset`, `task`, …), required on every entry. Free-form by default; declare a `types` vocabulary in `wiki.toml` to enforce a fixed set (`wiki check` then errors on any undeclared type). `wiki property type --counts` shows what's in use.
- **Tags** = everything cross-cutting (`2026`, `needs-review`, a task's `feature`/`bug`). If a thing would ever live in two folders, it's a tag, not a folder.

**One thing per entry.** Similar-but-distinct things (a `type` and its factory) each get their own entry, linked to each other, never folded into one. Being adjacent or similarly named is a reason to link, not to merge. Merge only true duplicates: two entries for the *same* thing.

**A pile of records is one entry, not many.** "One thing per entry" governs *knowledge* you link to and reason about. Its opposite is *uniform records* you mostly count, sum, or filter in bulk (invoices, expenses, transactions, sensor readings): those go in a single `type: dataset` entry as one Markdown table, one row per record, queried with `wiki table <file> --format csv | duckdb` (or `jq` over `--format json`). The test is whether you want `backlinks` to each one or `SUM`/`GROUP BY` over all of them: individual identity wants entries, aggregation wants a dataset. **Store each cell raw and machine-readable** from the first row (an unformatted number `1234.55`, an ISO date `2026-02-05`, any unit such as a currency in its own column), never display-formatted (`1,234.55 USD`): `wiki table` then feeds `duckdb`/`jq` with no cleanup, and formatting stays a presentation concern rather than something baked into the stored data (a formatted `"1,234.55"` is a string that won't sum, and forces a fragile `gsub` at query time). A table row is data, not a graph node (a cell may name `/clients/acme.md`, but that is text, not an edge `backlinks` follows); if one table grows unwieldy, partition it by year or owner (`finance/invoices/2026.md`). And since a dataset needs a `type`, it is always a typed file (`invoices.md`), never an `index.md` (which carries no frontmatter): a folder of yearly tables is a collection fronted by a typeless `invoices/index.md` that *links* the dataset files, it cannot *be* one.

Entries link with standard Markdown, relative to the linking file: `[Income](../finance/income.md)`, so they navigate in any renderer. Two conveniences, both normalized by `wiki tidy`: a hand-written root-absolute link (`/finance/income.md`) still resolves and `tidy --links` respells it relative; an Obsidian `[[wikilink]]` resolves too and `tidy --wikilinks` converts it to a standard Markdown link. Two reserved filenames carry no frontmatter at all (so no `type`): `index.md` (a folder's navigation surface) and the optional `log.md` (a dated chronicle); the one exception is the bundle-root `index.md`, which carries `okf_version`.

Since an `index.md` holds no frontmatter (no `type`, so no concept of its own), the two serve different jobs, and the deciding question is what the folder *is*.

- A **`thing.md` is load-bearing**: when a folder's contents are the parts of one thing (a project, a product, a plugin), that thing needs a home that can be typed, linked to, and found, so it gets a `thing.md` beside its `thing/` folder, never a typeless `thing/index.md`. 
- A **folder `index.md` is only the entry point into a collection**: it fronts a folder of otherwise-independent entries (the bundle root, `projects/`, `people/`) to navigate what's inside, and carries no concept of its own.

Ask which the folder is: one concept with parts (`thing.md`), or a bag of separate entries (`index.md`).

`wiki` only indexes `.md` files. Files matched by `wiki/wiki.toml`'s `ignore` list are operating docs, not wiki entries: absent from the index and from `check`. Paths matched by `ignore_orphans` stay full entries (indexed and `check`ed), only kept out of the `orphans` report (a parked backlog, say). So a file you want left out entirely (a `LEARNINGS.md`, a `README.md`) goes in `ignore`, not `ignore_orphans`. Both accept an exact path or a glob (`*`, `?`, `**`), so a single file (`notes/draft.md`) and a subtree (`archive/**`) both work.

### Get oriented

On a base you don't already know, look around first, and the fastest way to learn its shape, vocabulary, and health:

```sh
wiki status                                # entries, links, tags, tasks, broken, orphans
wiki list --sort=timestamp | head -n 40    # what changed recently (--reverse for the stalest)
wiki property type --counts                # what kinds of entries exist
wiki tags --counts --sort=count            # dominant topics
find -maxdepth 2 -type d | grep -v .git/
```

Then follow `/index.md` and its links down, rather than grepping in the dark.

### Division of labor

- **You** read, write, and edit Markdown directly: create entries, compose frontmatter, draft content, add links. This is the judgment work `wiki` can't do.
- **`wiki`** is the deterministic engine: queries, the link graph, moves, tidying, health. It never creates content.

A query is only as good as the indexing preceding it: an entry nothing links to is invisible to `backlinks`; a task never added to a board won't show in `tasks`. The tool reflects what is *indexed*, not what exists. Using `ls`, `find` or `grep` is the last resort.

### Recipes: a need, and the command that meets it

#### Find and recall

```sh
wiki search "docker networking"                # every word by default (AND), case-insensitive, over frontmatter + body
wiki search "docker networking" --any          # any word (OR): broadens the net
wiki search "docker networking" --exact        # the verbatim phrase (contiguous)
wiki search "docker" --lines                   # matching lines as file:line
wiki list --where type=concept --where tags=docker   # filter by kind and topic (repeatable = AND)
wiki list --where type=task --where status=done       # any frontmatter field, one flag
wiki list --where type=task --where status!=done      # key!=value negates (here: still-open work)
wiki list --where type=note --prefix personal/       # scope to a subtree
wiki read /tech/infra/docker.md                # an entry's body, frontmatter stripped
wiki outline /tech/infra/docker.md             # its heading map
wiki table /finance/expenses.md --format csv   # extract a dataset's table, for jq/duckdb
```

`--prefix <path>` scopes to a subtree and works on `list`, `search`, `checkboxes`, `tags`, `properties`, and `property`: reach for it to narrow any query in a large base. When the user asks something, check the base first: it may hold the answer, or reveal a gap worth a new entry. For structured questions ("all blocked tasks"), prefer `list --where status=blocked` (exact frontmatter-value match, repeatable for AND) over `search`, which matches query words as substrings anywhere in the file (frontmatter + body): by default every word must appear, in any order (broaden with `--any`, or narrow to a verbatim phrase with `--exact`). Use `key!=value` for negation (`status!=done` is everything still open, including entries with no `status` yet). Compare against the empty string to test emptiness: `--where assignee=` is entries with no assignee (unset or blank), and `--where assignee!=` is those that have one. It stays equality/inequality only; richer reporting is a skill over `--format json`.

#### Follow the graph (what grep can't do)

```sh
wiki links /index.md                  # what it points to
wiki backlinks /tech/infra/docker.md  # what points here
wiki unresolved                       # promised but not yet written: a to-write list
wiki orphans                          # nothing links in: knowledge to index
```

#### Capture → refine → promote

Knowledge often arrives rough and matures in place. You write the file; `wiki` only queries it.

1. **Capture** the rough thought as an entry now (don't lose it to a scratchpad).
2. **Refine**: read it back (`wiki read`), ask the user a sharpening question or two, fill it in.
3. **Promote**: give it a real `type`, `wiki move` it into its domain, and **link it in** from the domain's `index.md` and related entries. An unlinked entry is lost knowledge.

*(How this base holds unclassified thoughts, such as an inbox or a `draft` type, is a [WORKFLOW.md](./WORKFLOW.md) choice.)*

#### Reshape safely

```sh
wiki move --dry-run /a.md /archive/a.md   # preview the link rewrites
wiki move /a.md /archive/a.md             # relocate + rewrite every inbound link in one pass
wiki tidy                                 # preview canonicalization; `tidy --all` applies (links→relative, names→slugs)
```

Prefer `wiki move` over read-delete-rewrite by hand: hand-moving strands every backlink.

#### Track work

Two things wear a checkbox-ish shape; the model separates them by **who owns the state**:

- A **`type: task` entry** is a first-class thing that owns its own state (`status` frontmatter is the source of truth). List them with `wiki list --where type=task`.
- A **`- [ ]` checklist item** is a *subtask or step of the entry it lives in* (a task's steps, a note's to-dos), owned by that entry and nothing else. `wiki checkboxes` gathers open checklist items across the base, a `--prefix` subtree, or one `[file]`. (The command is `checkboxes`, not `tasks`: it scans the `- [ ]` construct, it does not list `type: task` entries.)

```sh
wiki checkboxes                 # open '- [ ]' items across the base
wiki checkboxes /active/login.md  # just that entry's own subtasks
wiki list --where type=task     # the task entries themselves
```

A **board** (`index.md`) references task entries with **plain links**, not by checkboxing them: a link is an external reference and never carries the target's state (the entry owns that, so a board checkbox would just be a second copy that drifts). A genuinely trivial to-do not worth its own entry can sit as a bare `- [ ]` on the board, but then it is a checklist item, not a queryable entry, that is the trade-off. The board is **authored, not generated**: `wiki` never edits it, you keep it current; a task the board omits shows up under `wiki orphans` if nothing else links it.

*(How this base runs a board (columns, priorities, pruning) is in [WORKFLOW.md](./WORKFLOW.md).)*

### Keep it healthy

**After any batch of edits, run `wiki check` before you're done.** It's the gate:

```sh
wiki check        # warnings (e.g. a broken link) exit 0; errors (missing/unknown type) exit 1
wiki check --fix  # apply the safe auto-repairs (e.g. okf_version sync)
```

Groom in small steps that compound: turn `unresolved` links into entries when it helps, re-home `orphans`, surface and merge true duplicates (two entries for one thing; similar-but-distinct entries stay separate and linked), consolidate redundant tags, link related entries, and keep each folder's `index.md` current. **Change something only when it makes the base more findable, never restructure for its own sake.** A broken link is tolerated (it's future knowledge, and `unresolved` lists it); `check` likewise warns on a `#anchor` link whose target has no such heading.

### Git and safety

Git is the undo for a base an agent edits. This base lives inside the project repo, so git covers it: pull before editing, and after `wiki check` passes, commit the batch with a clear message; resolve conflicts by preserving both sides' intent and merging frontmatter sensibly.

### Conventions

- Every entry has a `type` (reserved `index.md`/`log.md`); slug filenames (lowercase, hyphenated, no spaces); shallow folders (2–3 levels).
- Root-absolute links, no wikilinks: `wiki check` flags any `[[wikilink]]` (they're resolved into the graph for compatibility, but aren't part of the format), so rewrite them as standard Markdown links (or run `wiki tidy --wikilinks` to convert them).
- Every command that reports results takes `--format text|json|csv|tsv` (`json`/`csv`/`tsv` for structured output you can pipe; `version` prints a bare string). `list --format json` carries each entry's full frontmatter (every field, not just the shown columns), so `wiki list --where type=task --format json | jq …` is the reporting surface for rollups the CLI does not compute itself. text and csv/tsv show only the canonical columns, the entry's path and `type` (the two fields every entry is guaranteed to have); everything else (title, tags, status, …) is json-only.
- json keys follow the shape of what is being reported. `list` and `orphans` merge your frontmatter into each row, so the entry's own path is under the **reserved key `_path`** (leading underscore; the basename is just `basename(_path)`) and a frontmatter field named `name:` or `path:` round-trips untouched — every other key is your frontmatter verbatim. Rows *about* an entry (`check`, `checkboxes`) name it `entry`. Link rows (`links`, `backlinks`, `unresolved`) name their two ends `from` and `to`.
- Exit codes: `0` ok (enumerations return `0` even when empty), `1` no match (`search`/`table`) or `check` errors, `2` a real error.
- `wiki` not installed? `brew install agentic-wiki/tap/wiki`, or see the [wiki repo](https://github.com/agentic-wiki/wiki).
