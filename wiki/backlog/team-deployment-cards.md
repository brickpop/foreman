---
type: task
status: done
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Team → Deployments: a card opens its sessions

**Why:** the CEO, 2026-10-16. On the Team page, a deployment's card opens the project's wiki, and its sessions are behind a separate **Sessions** button. From a team, the CEO wants that team at work, not the project's wiki ([Team page → Deployments](../product/features/team-page.md#deployments)).

**Done when:**
- The whole card opens [Sessions](../product/features/sessions.md) on that deployment.
- Only the project name links to the project (the keyboard path for each stays a real link or button).
- The **Sessions** button goes, since the card does that now. **Stop** keeps its own action.

- [x] the Team page spec, then the card (2026-10-17: the sessions link is stretched over the card; the name and Stop sit above it)
- [x] a UI test: deferred by the CEO (2026-10-17) to [UI component tests](./ui-component-tests.md), since the UI has no component tests yet
