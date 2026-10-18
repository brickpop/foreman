---
type: task
status: todo
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

- [ ] **Remove the earlier design from the code:** the drawer (`ChiefOfStaffDrawer`, the `chiefOfStaff` client area and its types, `cos.changed`), Chief of Staff review requests (`source: cos`, `authorInstance === "cos"` in `RequestPanel`, `ReviewPanel`, Home, Changes, notifications; the `store` comments on requests and merge requests), and Project settings' `Change with Chief of Staff` button and its text
- [ ] the session: start / stop like the login container (`POST` / `DELETE /api/cos`), the `default` profile's harness, the mounts, `.foreman/cos/` git-ignored, `--continue` on the next visit, the opening line from the entry point, one session shared by every tab
- [ ] its prompt: who it is, the team file format and invariants, the roles' default job descriptions, the names in use, `propose_team`, the flows' shapes
- [ ] `propose_team` through the agent port: checked with the team file's own validation (the one a save runs), reasons returned when invalid, `cos.proposal` to the page; `POST /api/cos/proposal` writes it version-guarded (409), then tells the session
- [ ] the page: header, the terminal full width, the preview beside it (team, playbook, escalation, a tab per member, new or changed marked, the before of a changed field), `Adjust team`, `Show preview`, `Create` / `Save`; the entry points (`+ New team`, Hire, Refine, the sidebar tile)
- [ ] tests: an invalid proposal never reaches the page; a confirm after the team file changed writes nothing; a newer proposal replaces the older; a hire starts on every deployment of the team; the session mounts nothing of `.foreman/` but `cos/` and `teams/`; leaving the page stops it; end to end on the fake harness (a scripted CoS calling `propose_team`)
