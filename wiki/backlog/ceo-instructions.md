---
type: task
status: todo
priority: medium
tags: [feature]
epic: install-first-run
---

# The CEO's own instructions in every session

**Why:** the CEO, 2026-10-26. The CEO keeps standing instructions for coding agents in `~/.claude/CLAUDE.md` (Claude Code) and Pi's global `AGENTS.md`, and expects agents run by Foreman to follow them too. They don't, today:

- **Claude Code** runs with a private `CLAUDE_CONFIG_DIR` per instance, and Foreman writes that dir's `CLAUDE.md` with the [session prompt](../product/architecture/session-prompt.md) and nothing else (`internal/harness/claudecode/claudecode.go`). The host's `~/.claude` is never mounted: only the profile's login file is ([Session runner → Credentials](../product/architecture/session-runner.md#credentials)).
- **Pi** runs with a private `PI_CODING_AGENT_DIR`, where Foreman writes the session prompt as `APPEND_SYSTEM.md` (`internal/harness/pi/pi.go`). The host's `~/.pi/agent/` is never mounted.
- Not checked: whether a code repo's own `CLAUDE.md` or `AGENTS.md`, in an agent's worktree, is still read by the harness (Claude Code reads one in its working folder; Pi runs with `--no-approve`, which declines the repo's `.pi/`).

**Asked:** a place in Settings where the CEO writes instructions that Foreman adds to every session prompt, perhaps per profile.

**Done when:** the CEO's instructions reach every agent's session on its next start, the [Session prompt](../product/architecture/session-prompt.md), [Settings](../product/features/settings.md) and [Global settings](../product/models/global-settings.md) (or the profile's spec) describe it, and the "not checked" point above is answered.

**To decide with the CEO before building:**
- **Where they are set.** Global (one text for every agent, in Settings), per profile (Settings → Agents → Profiles, kept in `~/.foreman/profiles/<name>/`), or both, the profile's added after the global one. Per profile fits instructions that only one harness or model needs; the same rules usually apply whatever the harness, so global may be enough.
- **Written in Foreman, or read from the CEO's own files.** A text edited in Settings and kept in Foreman's home, or Foreman copying `~/.claude/CLAUDE.md` (or Pi's `AGENTS.md`) at each start. Copying keeps one source but reads files outside Foreman's home, includes instructions meant for the CEO's own sessions (paths on the Mac, tools not in the container), and a file the CEO didn't write for agents ends up in every sandbox.
- **Where in the prompt.** Before or after Foreman's own rules, and which wins when they conflict (Foreman's: the team tools and git policy can't be overridden by a personal note).
- **The other scopes.** Per team or per agent already exist in part (the soul's job description); per project is the project repo's `AGENTS.md`. Whether this is only the CEO's personal layer.
- **When it applies.** At the next start, like a profile change, or also told to running agents.

- [ ] decide the questions above with the CEO
- [ ] the setting, the prompt assembly, the UI
- [ ] tests: the text reaches the prompt on both harnesses; an empty one adds nothing; a change applies at the next start
