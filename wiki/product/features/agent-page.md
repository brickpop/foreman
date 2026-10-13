---
type: spec
status: planned
tags: [feature, ui, agents, teams]
---

# Agent page

One agent's **soul** ([Agent soul](../models/agent-soul.md)): who it is, what it runs on, and the exact session prompt each deployment gives it. Edits write the team file and apply on every project the agent runs on.

## Layout

Padding `40px 48px 64px`. Two columns (`repeat(auto-fit, minmax(360px, 1fr))`, gap 40).

## Left column

- **Header**: Avatar 72 (no dot), H1 name (26px), and `<Role> · <Team>` (`--tx3`; opens the team's Members tab, [Team page](./team-page.md)).
- **"Running on"**: one row per deployment (grid `auto 80px 1fr auto`, `--panel`, radius 9): an 8px status dot, the project (600), the current task (ellipsis), and a **`Session`** button ([Sessions](./sessions.md)). If the team isn't deployed: `Not running. <Team> isn't deployed to any project.`
- **"Look"**, with **`Shuffle`** (12.5px `--accTx` link) on the right ([Avatar](../design/avatar.md)):
  - Swatch rows (84px label column): `Background` (8), `Skin` (5), `Hair color` (5).
  - Trait rows of option buttons labelled with the values: `Hair` (short, long, curly, bun, bald), `Eyes` (dots, glasses, shades, sleepy), `Beard` (none, stubble, full, mustache).
  - Shuffle randomizes all traits and the background.
- **"Harness"**: option buttons `Claude Code`, `Codex CLI`, `Pi`, `Hermes Agent`, `Custom`. Custom shows a Mono command input (placeholder `./bin/my-agent --acp`). See [Harness adapters](../architecture/harness-adapters.md).
- **"Personality"**: textarea (3 rows, 15px).
- **"Skills"**: chips with `×`, and a `+ add` input (Enter adds; blank and duplicate skills are ignored).
- **"Job description"**: a Mono textarea (6 rows, `--term`). Its label shows `✓ Saved. Applies on every project.` (12.5px `--ok`) after a save.
- **`Refine with Chief of Staff`** (outline): the [Chief of Staff](./chief-of-staff.md) refine flow.

**Saving**: every field saves to the team file without a save button ([API](../architecture/api.md), `PUT /api/teams/:id/agents/:agent`). Running instances get a context-update message on their next turn; the full prompt changes at the next restart ([Agent runtime](../architecture/agent-runtime.md)). A save that conflicts with a change made on disk meanwhile shows an error and reloads the field.

## Right column

- "Session prompt" (600), subline: "Generated at session start from this soul plus the project. The soul is the same everywhere; the context isn't."
- A switcher `on <project>` (option buttons), shown when the agent runs on more than one deployment.
- The composed prompt in a `<pre>`, exactly as given to the harness ([Session prompt](../architecture/session-prompt.md), `GET /api/instances/:id/prompt`).

## Open questions

1. **Save timing and the "Saved" label.** The prototype saves on every keystroke and the label never clears. Each save rewrites a YAML file that agents and the CoS also read, and each one triggers a context update. Options: save on blur / Enter (text fields) and immediately (buttons, swatches); debounce ~800 ms. Recommendation: save on blur for text, immediately for choices; the label shows for 3 s; one context update per save, coalesced per turn.
2. **Where `look` is stored.** PRD §5.4 stores it in the team file under each agent's `look:`; the prototype's File view omits it. Recommendation: in the team file, as the PRD says (it's part of the soul).

The prompt shown when the team isn't deployed is owned by [Session prompt](../architecture/session-prompt.md#open-questions).
