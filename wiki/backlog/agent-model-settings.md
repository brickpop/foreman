---
type: task
status: backlog
priority: medium
tags: [feature]
epic: install-first-run
---

# Model, effort and auto-compact per agent

**Why:** the CEO, 2026-10-18. Every agent runs on its harness's defaults: the same model, the same reasoning effort and the same auto-compact threshold for a lead and for a QA. The CEO wants to choose those per agent.

**To decide when it's scheduled:**
- **Where it is set.** In the soul (`.foreman/teams/`, travels with the team, the Chief of Staff can propose it), or per project in Project settings (like the profile, kept out of what the lead can write)? Probably the soul, with the project able to override.
- **What the values are.** Each harness names models and effort its own way: Claude Code (`--model`, its effort setting), Pi (`--model provider/id`, `--thinking off…max`, compaction in `settings.json`). Either free text that the adapter passes through, or a list each adapter offers (`GET /api/harnesses`). A value the harness doesn't know fails at the agent's start: how that shows.
- **Auto-compact threshold:** Claude Code reads it from an environment override; to verify on the current version. Pi has `compaction` settings (reserve and keep-recent tokens, not a percentage). One field meaning "compact at N % of the context" may not map onto both.
- **When it applies:** at the next start, like the profile.

**An idea (the CEO, 2026-10-18), on the radar, not decided:** a profile holds several named presets (name to find: modes, presets…), each a model, an effort and a threshold, and an agent picks one. For Pi, a preset could also carry the `models.json` the CEO templates from their own providers and URLs ([Pi harness](./pi-harness.md)).

Close to [Profiles per agent](./profiles-per-agent.md) (another harness or account per agent): decide both together, since a model only makes sense for its harness.

- [ ] decide the questions above with the CEO
- [ ] the fields, the adapters passing them, the UI, tests
