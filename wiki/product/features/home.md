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

One **request panel** per open [request](../models/request.md), across all projects (gap 10). The kinds, their actions and their effect are defined in [Requests](./requests.md). Home and the project's Overview ([Needs you](./project-pipeline.md#1-needs-you)) use the same request panel component, so they can't drift: approvals and questions carry the free-text answer field here too. The differences below are only the extra context Needs you adds (source line, PR link).

- **Request panel**: `--panel`, radius 12, padding `18px 20px`, gap 14. The whole panel opens its agent's session ([Sessions](./sessions.md)); its buttons, links and answer field keep their own action, and selecting text opens nothing. A Foreman-raised request (no agent) opens nothing.
- **Header row**: Avatar 32 with an amber ring ([Avatar](../design/avatar.md)), the agent name (600; avatar and name are a button doing the same, the keyboard path), the context (13px `--tx4`): `<Team> on <project>`; a spacer; the kind chip.
- **Text**: the request summary, 16px, line-height 1.5.
- **Buttons** (wrap, gap 8), styled by position ([Design system](../design/design-system.md#buttons)):
  - Approval: its options (e.g. `Approve` / `Deny`). No `Open session` button: the card itself opens the session.
  - Question, finding, blocked: its options.
  - Review (a merge request): decided **right here**, like a review on the project's Overview, under [Needs you](./project-pipeline.md#1-needs-you) (the CEO, 2026-10-17: merging is a fundamental action, not one to look for in a project's tabs): the text `<Lead> wants <entry title> merged into <main> in <repo>.` (with `Checks pass.` once checks run), its link line (opens the change), **`Merge to <main>`**, **`Create PR`** (when its repo has a forge), **`Reject`** (with a message to the lead). `<main>` is the project's protected branch ([Pipeline and merges](./pipeline-and-merges.md#review-and-merge)).
- Choosing an option resolves the request ([Requests](./requests.md)). [Needs you](./project-pipeline.md#1-needs-you) keeps an `Open session` button, since its cards are not clickable as a whole.
- **Order**: blocked first, then approvals, reviews, questions and findings; within a kind, oldest first.
- **Empty**: dashed box `Nothing needs you. You'll get a browser notification when something does.`

## Projects list

Label "Projects", then one row per project (grid `1fr auto`, padding `14px 4px`, top border). A click opens the project's [Wiki reader](./wiki-reader.md).

- Name (600 15px).
- Line (13.5px `--tx3`): `<Team>, <Team> · <n> agents working`, or `No team deployed`.
- State (13px): `needs you` (`--accTx`) if it has open requests, `on track` (`--ok`) if any instance is running, otherwise `idle` (`--tx4`).
