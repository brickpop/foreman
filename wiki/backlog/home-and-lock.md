---
type: task
status: in-progress
priority: high
tags: [feature]
epic: projects-anywhere
---

# The home and the lock

**Why:** one Foreman per machine, at `~/.foreman/` ([Projects anywhere](./epics/projects-anywhere.md)): two servers on overlapping folders, or on the same one, overwrite each other today, and nothing stops them.

**Scope, as built (2026-10-20):** the home, its lock and the per-machine data moved in. The workspace folder argument stays until [Projects as records](./projects-as-records.md), which removes it; teams and settings move into the home's global repo with that card too (they live in the workspace repo until projects stop living there).

**Done when:**
- Foreman's home is `~/.foreman/` (`--home <dir>` for tests and `just dev`, which uses its workspace's `.foreman/` and links the CEO's profiles and images in), created on the first run without a question.
- It holds an OS lock on `~/.foreman/server.lock` (0600) while it runs (`flock`, released by the system on any exit, so never stale), with the URL it serves and its workspace; a second `foreman` prints them, opens the URL (unless `--no-open`), and exits.
- Profiles, preferences and image builds move from `~/.local/share/foreman` (or `$XDG_DATA_HOME/foreman`) to the home, once, on the first start that finds them there, never overwriting what the home has.
- Tests: the lock (a second start pointed at the first, a dead process's lock gone), the move (once, never overwriting, nothing to move).

- [x] the home and `--home`; the lock
- [x] per-machine data moved in
- [x] specs: Persistence, Session runner, README; the epic's side effects
- [ ] the CEO's check: the first real start moves `~/.local/share/foreman` in, and a second `foreman` opens the first
