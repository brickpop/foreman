---
type: spec
status: planned
tags: [feature, ui, requests]
---

# Home

The CEO's inbox: everything that needs a decision across all projects, then a one-line state per project. The goal is to make the CEO's job small: see it, decide, move on.

## Header

- Title (page H1): `N things need you`, `1 thing needs you`, or `All clear` when there are no open requests.
- Subtitle: `<running> sessions working across <deployments> deployments.` Running = instances with status `running` ([Instance](../models/instance.md)); deployments = active [deployments](../models/deployment.md).

## Setup checklist

While any setup step is left, Home starts with the setup checklist, above the requests: Podman, its machine (macOS), the agent image, each harness login, a team, a project, a code repo per project, each with its button ([First run](./first-run.md#the-checklist)). It goes away once every step is done.

## Request list

One **request panel** per open [request](../models/request.md), across all projects (gap 10). The kinds, their actions and their effect are defined in [Requests](./requests.md). Home and [Changes](./changes.md) use the same request panel component, so they can't drift: approvals and questions carry the free-text answer field here too. The differences below are only the extra context Changes adds (source line, PR link).

- **Request panel**: `--panel`, radius 12, padding `18px 20px`, gap 14. The whole panel opens its agent's session ([Sessions](./sessions.md)), or the Chief of Staff for its items; its buttons, links and answer field keep their own action, and selecting text opens nothing. A Foreman-raised request (no agent) opens nothing.
- **Header row**: Avatar 32 with an amber ring ([Avatar](../design/avatar.md)), the agent name (600; avatar and name are a button doing the same, the keyboard path), the context (13px `--tx4`): `<Team> on <project>`, or `on <project>` for a Chief of Staff item; a spacer; the kind chip.
- **Text**: the request summary, 16px, line-height 1.5.
- **Buttons** (wrap, gap 8), styled by position ([Design system](../design/design-system.md#buttons)):
  - Approval: its options (e.g. `Approve` / `Deny`). No `Open session` button: the card itself opens the session.
  - Question, finding, blocked: its options.
  - Review (a merge request): text `<Lead> wants <entry title> merged into main in <repo>. Checks pass.` and one primary **`Review`** button, which opens Project → [Changes](./changes.md), where the CEO chooses Merge to main, Create PR on the forge, or Reject ([Pipeline and merges](./pipeline-and-merges.md#review-and-merge)).
  - Chief of Staff item: reviewed right here, like a review in [Changes](./changes.md#1-needs-you): its link line (opens the change), **`Merge to main`**, **`Create PR`** (when its repo has a forge), **`Reject`** (with an optional message back to the Chief of Staff); the card itself opens the Chief of Staff. A change to a team file has no project, so its context is empty.
- Choosing an option resolves the request ([Requests](./requests.md)). [Changes](./changes.md) keeps `Open session` and `Open Chief of Staff` buttons, since its cards are not clickable as a whole.
- **Order**: blocked first, then approvals, reviews (Chief of Staff config reviews included), questions and findings; within a kind, oldest first.
- **Empty**: dashed box `Nothing needs you. You'll get a browser notification when something does.`

## Projects list

Label "Projects", then one row per project (grid `1fr auto`, padding `14px 4px`, top border). A click opens the project's [Wiki reader](./wiki-reader.md).

- Name (600 15px).
- Line (13.5px `--tx3`): `<Team>, <Team> · <n> agents working`, or `No team deployed`.
- State (13px): `needs you` (`--accTx`) if it has open requests, `on track` (`--ok`) if any instance is running, otherwise `idle` (`--tx4`).
