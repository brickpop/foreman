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

## Request list

One **request panel** per open [request](../models/request.md), across all projects (gap 10). The kinds, their actions and their effect are defined in [Requests](./requests.md). Home and [Changes](./changes.md) use the same request panel component, so they can't drift: approvals and questions carry the free-text answer field here too. The differences below are only the extra context Changes adds (source line, PR link).

- **Request panel**: `--panel`, radius 12, padding `18px 20px`, gap 14.
- **Header row**: Avatar 32 with an amber ring ([Avatar](../design/avatar.md)), the agent name (600), the context (13px `--tx4`): `<Team> on <project>`, or `on <project>` for a Chief of Staff item; a spacer; the kind chip.
- **Text**: the request summary, 16px, line-height 1.5.
- **Buttons** (wrap, gap 8), styled by position ([Design system](../design/design-system.md#buttons)):
  - Approval: its options (e.g. `Approve` / `Deny`), then **`Open session`** in the ghost style.
  - Question, finding, blocked: its options, then `Open session`.
  - Review (a PR to `main`): text `<Lead> opened PR #<n>: <title> to main. Checks pass.` (without a forge: `<Lead> wants <branch> merged into main.`) and one primary **`Review`** button, which opens Project → [Changes](./changes.md). The CEO merges on the forge (or with git), never in Foreman ([Pipeline and pull requests](./pipeline-and-prs.md#review-and-merge)).
  - Chief of Staff item: its options, then `Open Chief of Staff`.
- Choosing an option resolves the request ([Requests](./requests.md)). `Open session` opens the agent's session in [Sessions](./sessions.md).
- **Order**: blocked first, then approvals, reviews (Chief of Staff config reviews included), questions and findings; within a kind, oldest first.
- **Empty**: dashed box `Nothing needs you. You'll get a browser notification when something does.`

## Projects list

Label "Projects", then one row per project (grid `1fr auto`, padding `14px 4px`, top border). A click opens the project's [Wiki reader](./wiki-reader.md).

- Name (600 15px).
- Line (13.5px `--tx3`): `<Team>, <Team> · <n> agents working`, or `No team deployed`.
- State (13px): `needs you` (`--accTx`) if it has open requests, `on track` (`--ok`) if any instance is running, otherwise `idle` (`--tx4`).
