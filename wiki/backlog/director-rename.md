---
type: task
status: todo
priority: high
tags: [chore]
epic: director
---

# Chief of Staff → Director

**Why:** the CEO, 2026-11-05: the Chief of Staff becomes the [Director](./epics/director.md), the one agent above the teams. Renamed first, before anything is built on it, so nothing is renamed twice. Behaviour doesn't change in this card: it is still the page-bound session with `propose_team`; [The Director as a standing agent](./director-standing.md) changes how it runs.

**Renames, everywhere (no compatibility shims: Foreman is unreleased):**

- the name in the UI, the specs, the glossary, the vision's roles, the journey and the use cases;
- the spec `product/features/chief-of-staff.md` → `director.md` (`wiki move`), and the cards `cos-create-team`, `cos-project-config` → `director-…`;
- `internal/cos` → `internal/director`; the id `cos` → `director` (the agent port, `/ws/pty/director`, the process name);
- the route `/chief-of-staff` → `/director`, its `?start=` entry points kept;
- `~/.foreman/cos/` → `~/.foreman/director/`. The old folder's conversation is dropped, not migrated: the CEO deletes `~/.foreman/cos/` (the move script of [One Foreman folder](./foreman-folder.md) does it).

**New team:** opens the Director with the opening line `Help me create a new team.` (the CEO, 2026-11-08; was `Create a new team.`).

**Open, for the CEO:** [A Teams page](./teams-page.md) exists because `+ New team` starting the Chief of Staff's harness at once was "a bit too invasive" (2026-10-20). With the Director standing (it runs anyway), is opening it from New team still invasive, or does that card's motive go?

**Steps:**

- [ ] code: package, ids, route, folder, UI strings, the opening line
- [ ] specs and cards, with `wiki move` for the files
- [ ] tests renamed and passing; `rg -i "chief of staff|\bcos\b"` finds nothing but history
