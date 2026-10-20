---
type: task
status: todo
priority: medium
tags: [feature]
epic: chief-of-staff
---

# A Teams page: manage, create, delete

**Why:** `+ New team` in the sidebar starts the [Chief of Staff](../product/features/chief-of-staff.md)'s harness at once, with a prompt: too much for a click that may only be a look around (the CEO, 2026-10-20: "a bit too invasive"). And a team can't be deleted from the UI. The [app shell](../product/features/app-shell.md) says there is no Teams index screen; this card brings one. Soon, not urgent.

**Idea, to settle when it's picked up:**
- The sidebar's **Teams** label links to a Teams page: one row per team (members, where it's deployed), and the team's actions.
- **New team**, **Hire** and **Refine** open the Chief of Staff from there, on the CEO's explicit click, never by arriving on the page.
- **Delete a team**: refused while it's deployed (stop it first); otherwise one commit removing `teams/<team>.toml` from the global repo, so git keeps it. What becomes of its stopped deployments' history is to decide.
- Whether `+ New team` stays in the sidebar, pointing at the page instead.

- [ ] the page, the sidebar link, the Chief of Staff on a click
- [ ] delete, with its refusal and its commit; tests
- [ ] specs: App shell, Team page, Chief of Staff
