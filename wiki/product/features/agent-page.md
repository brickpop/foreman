---
type: spec
status: planned
tags: [feature, ui, agents, teams]
---

# Agent page

One agent's **soul** ([Agent soul](../models/agent-soul.md)): who it is, what it runs on, and the exact session prompt each deployment gives it. Edits write the team file and apply on every project the agent runs on.

## Layout

Padding `24px 24px 48px`. Two columns (`repeat(auto-fit, minmax(360px, 1fr))`, gap 40).

## Left column

- **Header**: H1 name (26px), and `<Role> · <Team>` (`--tx3`; opens the team's Members tab, [Team page](./team-page.md)).
- **"Running on"**: one row per deployment (grid `auto auto 1fr auto`, `--panel`, radius 9): an 8px status dot, the project's readable name ([App shell](./app-shell.md#sidebar); 600, never wrapped), the current task (ellipsis), and a **`Session`** button (the project's [Session](./sessions.md) on this agent). If the team isn't deployed: `Not running. <Team> isn't deployed to any project.`
- No harness choice (the CEO, 2026-10-18): an agent runs on its project's [profile](../architecture/session-runner.md#credentials), chosen in [Project settings](./project-settings.md); see [Agent soul → What it runs on](../models/agent-soul.md#what-it-runs-on).
- **"Personality"**: textarea (3 rows, 15px).
- **"Skills"**: chips with `×`, and a `+ add` input (Enter adds; blank and duplicate skills are ignored).
- **"Job description"**: a Mono textarea (6 rows, `--term`). Its label shows `✓ Saved. Applies on every project.` (12.5px `--ok`) for 3 s after a save.
- **`Refine with Chief of Staff`** (outline): the [Chief of Staff](./chief-of-staff.md#the-page), to refine this agent.

**Saving**: no save button. Text fields (personality, skills input, job description) save on blur or Enter; removing a skill saves immediately. Each save writes the team file ([API](../architecture/api.md), `PUT /api/teams/:id/agents/:agent`) and shows `✓ Saved. Applies on every project.` for 3 s. Each instance gets the new prompt at its next start; running ones aren't told ([Team definition → Reload](../models/team-definition.md#reload)). A save that conflicts with a change made elsewhere meanwhile (the CEO's editor, another tab, a Chief of Staff proposal saved) never drops the edit: it stays in the field and a prompt offers **Keep mine** (saves it over the new version) or **Take theirs** (drops it). Low priority.

## Right column

- "Session prompt" (600), subline: "Generated at session start from this soul plus the project. The soul is the same everywhere; the context isn't."
- A switcher `on <project>` (option buttons), shown when the agent runs on more than one deployment.
- The composed prompt in a `<pre>`, exactly as given to the harness ([Session prompt](../architecture/session-prompt.md), `GET /api/instances/:id/prompt`).

When the team isn't deployed anywhere, the full template is shown with `{project}`, `{sandbox}`, `{repos}`… as highlighted placeholders (`--accTx` on `--accSoft`), and the switcher is hidden ([Session prompt](../architecture/session-prompt.md)).
