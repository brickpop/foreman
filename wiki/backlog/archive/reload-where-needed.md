---
type: task
status: done
priority: high
tags: [bug, ui]
epic: workflow
---

# Reload shows where the CEO needs it

**Why:** "I fail to see the button to reload a deployment's team when/where I actually need it. All I see is a way to Stop the team" (the CEO, 2026-11-03). [Reload](../../product/models/team-definition.md#reload) is built, but hidden unless some live agent is `behind`, only in the project's Team header (`ui/src/screens/project/ProjectTeam.tsx`), and a run with no `prompt_hash` (started before 2026-11-02) is never `behind` (`internal/instances/launch.go`, `behind`), so a team deployed before then never shows it.

**Decided (the CEO, 2026-11-03),** written into [Team definition → Reload](../../product/models/team-definition.md#reload), [Project team](../../product/features/project-team.md), [Team page](../../product/features/team-page.md#deployments) and [Instance](../../product/models/instance.md):
- Always shown beside Stop while the deployment is active, disabled with the reason (`Every agent runs on the current prompt.`, or who is busy).
- In the project's Team header and on the Team page's deployment rows. In the Session only while some agent is behind.
- Foreman isn't released: no special case for old runs. A run with no hash is behind.

- [x] `behind`: the `PromptHash != ""` guard dropped; `TestAChangeLeavesLiveAgentsBehindUntilTheyStartAgain` checks the current hash is caught up and no hash is behind
- [x] the button always shown in the Team header, the reason line for every state (`reloadLine`)
- [x] the same on the Team page's deployment rows, from the one `reloadOf` / `reloadLine` (`ui/src/app/model.ts`)
- [x] the Session: shown while some agent is behind
- [x] tests: `reloadLine` for nothing behind (`app.test.ts`), the busy and behind lines as before; the no-hash run counts (Go). The button's placement has no component test ([UI component tests](../ui-component-tests.md))
- [x] specs as built (drop the "until built" notes)
