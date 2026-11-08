---
type: task
status: blocked
priority: medium
tags: [feature]
epic: director
---

# The Director proposes deployments

**Why:** the CEO, 2026-11-05: an outcome may involve a project with no team, or the wrong one. The Director proposes; the CEO's click deploys, as with team files.

Blocked by [The Director and the leads](./director-leads.md).

**Decided:**

- **`propose_deploy(project, team)`** and **`propose_stop(project)`** (Director only). Each opens a confirmation on the Director's page with the [Deploy](../product/features/deploy-team.md) or Stop dialog's own wording, the "this replaces the team on it" warning included (one team per project). Nothing happens until the CEO's **Deploy** / **Stop**.
- Checked as the dialogs check (the team exists, the project isn't archived, Podman ready); an invalid one returns the reasons to the Director instead of showing.
- Once done (or declined, or a newer proposal replaced it), the Director is told in its session (`From Foreman: the CEO deployed Atlas on ledger.`).

**Steps:**

- [ ] the tools, reusing the deploy and stop paths
- [ ] the confirmation on the Director's page
- [ ] specs: the Director's, [Deploy a team](../product/features/deploy-team.md)
- [ ] tests: nothing deploys without the click; the replacement warning; a refused proposal returns its reasons; the Director hears the outcome
