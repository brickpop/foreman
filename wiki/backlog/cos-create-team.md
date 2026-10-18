---
type: task
status: in-progress
priority: high
tags: [feature]
epic: chief-of-staff
---

# Chief of Staff: create a team, hire, refine

**Why:** creating teams by editing TOML by hand is slow (the CEO, 2026-10-18). The [Chief of Staff](../product/features/chief-of-staff.md) drafts a team from a description in its terminal, shows it in a preview, and the CEO's click writes it. Above anything else in the backlog (the CEO, 2026-10-18).

**Decided (the CEO, 2026-10-18):**
- **Teams only.** No requests, no Home items, no part in the agents' workflow; synchronous, the CEO at the keyboard.
- **A real terminal**, running the `default` profile's harness, on its own **page** (`/chief-of-staff`), in a 1-2-3 flow: talk, preview (one tab per member, `Adjust team` / `Create <Name>`), create.
- **An ad-hoc session like the login container**, not an agent of a deployment: started on opening the page, stopped on leaving it; its own `.foreman/cos/` (conversation carries over) and `.foreman/teams/` read-only, nothing else of `.foreman/`.
- **One tool, `propose_team`**: the whole team as it should be (new, a hire, a refine). Foreman checks it and shows it; only the CEO's click writes it, one commit. No branches or merge requests.
- `project.toml` changes are out: [Project config with the Chief of Staff](./cos-project-config.md). Starter or example teams are open: [Starter teams](./starter-teams.md).

**Done when:** from `+ New team`, a described team is drafted in the terminal, previewed member by member, adjusted, and created with one click; Hire and Refine work the same way; the old drawer and its demo code are gone.

- [x] **Remove the earlier design from the code:** the drawer (`ChiefOfStaffDrawer`, the `chiefOfStaff` client area and its types, `cos.changed`), Chief of Staff review requests (`source: cos`, `authorInstance === "cos"` in `RequestPanel`, `ReviewPanel`, Home, Changes, notifications; the `store` comments on requests and merge requests), and Project settings' `Change with Chief of Staff` button and its text. Done 2026-10-18: the entry points open `/chief-of-staff` (the sidebar tile removed: `+ New team` is the entry; the unused `Toggle` too) (`?start=create-team|hire|refine&team=&agent=`), which says "Not built yet"; `forgeOf` and `linkLine` lost their Chief of Staff cases (merge requests are on code repos only); the board's side sheet is the only drawer left
- [x] the session (`cmd/foreman/cos.go`): started and stopped like a profile's login, on the `default` profile's harness, mounts `.foreman/cos/` (rw) and `.foreman/teams/` (ro) and `foreman-agent` only, id `cos` on the agent port (`agentCalls` routes it to its tool; its token is live while it runs), `--continue` with the opener on every visit (`harness.Session.OpenerOnContinue`), a running session gets a new entry point's opener typed in, `cos/` added to older workspaces' `.gitignore`; refused while Podman isn't ready (NOT_READY) or the profile isn't connected (NOT_CONNECTED)
- [x] its prompt (`cos.Prompt`): who it is, how a change reaches the files, the three flows, the team file format, the built-in roles with their default jobs (`prompt.Roles`, `prompt.DefaultJob`), the ids and names taken
- [x] `propose_team` (`internal/cos`): strict JSON, checked by `Workspace.CheckTeam` (the same check `SaveTeam` runs, now also refusing a team id that isn't a slug), a deployed team's members can't be removed, `cos.proposal` to the page; `POST /api/cos/proposal` writes it version-guarded (409 STALE / CONFLICT), then tells the session
- [x] the page (`screens/ChiefOfStaff.tsx`): header with the profile's harness, the terminal, the preview beside it (team, playbook and escalation through the shared `TeamPlaybook`, a tab per member, new or changed marked, a changed field's old value, who leaves), faces drawn for members without one, `Adjust team`, `Show preview`, `Create` / `Save`; the stop a second after leaving, at once on closing the tab
- [x] tests: `internal/cos` (shown then created, only the one on screen, a change meanwhile writes nothing, refusals never shown, the prompt), `internal/workspace` (`CheckTeam`, the `.gitignore` upgrade), `internal/server` (the routes and their errors), `cmd/foreman` (the mounts and nothing else, the opener, a second entry point, stop and restart, NOT_READY, a proposal through the agent calls to a real team file), the UI (entry links, openers, what a proposal changes)
- [ ] **check on a real Claude Code login** (the CEO): that `claude --continue "<opener>"` continues and takes the line, and a whole create, hire and refine; the race tests (`just race`, no C compiler in the build container)
- [ ] an end-to-end run on the fake harness through the agent port (`tool propose_team {…}` typed into it): the integration test uses a stand-in script and calls the tool directly
