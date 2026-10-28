---
type: task
status: in-progress
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

**Decided (the CEO, 2026-10-28):** one global text in Settings → Agents, written in Foreman (not copied from `~/.claude/CLAUDE.md`), kept as `instructions` in `settings.toml`, saved with an explicit Save; last in the prompt, Foreman's rules winning on a conflict; the CEO's personal layer only; team agents only, not the Chief of Staff; at an agent's next start. Written into [Settings](../product/features/settings.md#instructions-for-every-agent), [Session prompt](../product/architecture/session-prompt.md#the-ceos-instructions) and [Global settings](../product/models/global-settings.md).

**Built (2026-10-28):**
- `home.Settings.Instructions`; `encodeSettings` writes it as a TOML `"""` string (the library writes every string on one line), escaping backslashes, control characters and any quotes that could close it.
- `prompt.Input.Instructions`, last in `Compose`; `Template` takes them too, so the Agent page shows them; `instances.compose` reads them from the settings at each start.
- The UI: the section in `AgentsTab.tsx`; `useSave` resolves to whether it saved; a settings change refetches the prompts shown.

- [x] decide the questions with the CEO
- [x] the setting, the prompt assembly, the UI
- [x] tests: the section last and as written, none when blank, in the template (`prompt`); a round trip of awkward text, multi-line in the file, absent when empty (`home`); reaching the harness at a start (`instances`); the prompts refetched (`staleKeys`). The Save/Discard behaviour waits for [UI component tests](./ui-component-tests.md)
- [ ] the "not checked" point above: whether a code repo's own `CLAUDE.md` / `AGENTS.md` in an agent's worktree is still read. The harness runs in the sandbox root, with each repo a subfolder: Claude Code loads a subfolder's `CLAUDE.md` when it reads files there; Pi, to check in a real run
- [ ] tried by the CEO (only the tests and the build have run it)
