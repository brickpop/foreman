---
type: task
status: todo
priority: medium
tags: [feature]
epic: chief-of-staff
---

# The lead proposes changes to its own team

**Why:** once a team is deployed, the CEO tunes it in the conversation with its lead, who then has the whole context. Changing the team's prompts today means going to the [Chief of Staff](../product/features/chief-of-staff.md) and explaining it all again, or asking the lead to draft a prompt to paste there (the CEO, 2026-11-02).

**Decided (the CEO, 2026-11-02),** written into [Team definition → Proposals](../product/models/team-definition.md#proposals) and [→ Reload](../product/models/team-definition.md#reload):
- Only the lead, about its own team: souls, playbook, escalation, purpose, hiring. Never the team id, removing members or another team.
- Its proposal opens a pane beside its terminal in the Session, like the CoS's, with **Approve** / **Reject** and a red/green diff of every changed field. The CEO's approval is always required.
- The CoS's main job becomes creating teams; amending through it stays allowed.
- A change applies at each agent's next start by default. On approving, the pane offers **Reload now?** when no agent of the deployment is busy; when one is, it says the change applies at the next start, to reload once they're idle (no turn is ever cut).
- A member hired into an active deployment gets its instance on save, asleep, with no reload.
- The never-built context-update notice is retired from the specs: next start or reload is the one way.
- Reload lives as a `Reload` button in the project's Deployment header (and as **Reload now?** on approving), shown whenever a live agent runs on an older prompt, whoever changed it (the team file, project.toml, the CEO's instructions). Reload, not Stop and redeploy, applies a change to a running team. Stop and redeploy keeping conversations is its own card: [Continue conversations on redeploy](./continue-conversations-on-redeploy.md).

**How it stands today (checked 2026-11-02):** a team file save is only published to the UI (`cmd/foreman/main.go`, the `home.Watch` callback has no `team` case), and each prompt is composed at spawn (`instances.compose`), so nothing running is disrupted. Instances are made only in `Manager.Deploy` (`internal/instances/lifecycle.go`), so a member hired into a deployed team, through the CoS too, gets no instance until the team is stopped and deployed again: a bug of the CoS's Hire as it is, fixed by giving the hire its instance on save.

**Already there, to reuse:** `cos.Proposal`, `Home.CheckTeam`, `SaveTeam` with its version guard, the deployed-members check, `POST /api/cos/proposal`, the preview in `screens/ChiefOfStaff.tsx`, the CEO's Restart (`--continue`).

- [ ] proposals held per proposer, not one global one (the CoS and every lead can each have one on screen)
- [ ] the tool on the lead's side of the agent port, scoped to its own team, plus a way to read its team file as it is now (its prompt holds only its own soul)
- [ ] the diff: per field, removed text red, added green; the preview extracted into one component, used by the CoS page and the Session pane
- [ ] the Session pane: Approve / Reject, the lead told either way
- [ ] a hire gets its instance on save, on every active deployment of the team (fixes the CoS's Hire too)
- [x] reload (done 2026-11-02): each start keeps its prompt's hash (`instances.prompt_hash`, migration 013), `GET /api/instances` marks the live ones `behind`, and the Deployment header's `Reload` restarts them, disabled while any agent is queued, starting, running or awaiting; tests in `internal/instances` (behind after a soul edit and the CEO's instructions, caught up by a restart, asleep never, no hash never) and `app.test.ts` (`reloadOf`, `reloadLine`, the refetch on config changes)
- [ ] **Reload now?** in the approval pane, from the same `reloadOf`
- [ ] tests: own team only, members refused, a stale proposal writes nothing, members can't be removed, a refused proposal is never shown, the diff marks every changed field and nothing else, a change reaches a member at its next start, a hire gets its instance on save and nothing running restarts
- [ ] specs updated as built: [Project deployment](../product/features/project-deployment.md), [Team definition](../product/models/team-definition.md), [Chief of Staff](../product/features/chief-of-staff.md), [Sessions](../product/features/sessions.md), [Protocol](../product/architecture/protocol.md), and the lead's prompt in [Session prompt](../product/architecture/session-prompt.md)
