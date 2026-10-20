---
type: task
status: in-progress
priority: high
tags: [feature]
epic: projects-anywhere
---

# The home and the lock

**Why:** one Foreman per machine, at `~/.foreman/` ([Projects anywhere](./epics/projects-anywhere.md)): two servers on overlapping folders, or on the same one, overwrite each other today, and nothing stops them.

**Scope, as built (2026-10-20):** the home (with the per-machine data) and its lock. The workspace folder argument stays until [Projects as records](./projects-as-records.md), which removes it; teams and settings move into the home's global repo with that card too (they live in the workspace repo until projects stop living there).

**Done when:**
- Foreman's home is `~/.foreman/` (`--home <dir>` for tests and `just dev`, which uses its workspace's `.foreman/` and links the CEO's profiles and images in), created on the first run without a question.
- It holds an OS lock on `~/.foreman/server.lock` (0600) while it runs (`flock`, released by the system on any exit, so never stale), with the URL it serves and its workspace; a second `foreman` prints them, opens the URL (unless `--no-open`), and exits.
- Profiles, preferences and image builds live in the home. Nothing is moved from the old `~/.local/share/foreman`: Foreman was never released (the CEO, 2026-10-20), so the CEO moves their own folder by hand once.
- Tests: the lock (a second start pointed at the first, a dead process's lock gone).

- [x] the home and `--home`; the lock
- [x] per-machine data in the home
- [x] specs: Persistence, Session runner, README; the epic's side effects
- [ ] the CEO's check: `mv ~/.local/share/foreman/{profiles,images,preferences.toml} ~/.foreman/`, then a second `foreman` opens the first
