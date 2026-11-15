# Product

Foreman's product definition. Every entry carries a `status` (`planned` / `partial` / `built`); see `WORKFLOW.md` at the project root. Open questions live in each entry's `## Open questions` section.

- [Vision](./vision.md): summary, goals, non-goals, roles, non-functional requirements
- [Glossary](./glossary.md)
- [The end-to-end journey](./journey.md): one story through everything, step by step, with the blocks behind each step
- [Use cases](./use-cases.md): the end-to-end scenarios v1 must pass

## Data models

Files Foreman reads or owns, and the runtime state it keeps.

- [Home](./models/home.md): `~/.foreman`, its layout and the config / state / content split
- [Project config](./models/project-config.md): the project record and `project.toml`
- [Team definition](./models/team-definition.md) (`config/teams/<team>.toml`)
- [Agent soul](./models/agent-soul.md)
- [Global settings](./models/global-settings.md) (`config/settings.toml`)
- Runtime state (`state.db`)
  - [Deployment](./models/deployment.md)
  - [Instance](./models/instance.md)
  - [Request](./models/request.md)
  - [Team message](./models/team-message.md)
  - [Work item](./models/work-item.md): an agent's branch on its way to the lead
  - [Merge request](./models/merge-request.md): the lead's proposal to merge into one of the CEO's branches
  - [Review](./models/review.md): the CEO's review of a lead branch, with its comments to the lead
  - [UI state](./models/ui-state.md): read markers and UI preferences

## Features

- [App shell](./features/app-shell.md): sidebar, toasts, drawers, dialogs
- [Dashboard](./features/dashboard.md)
- [First run](./features/first-run.md): Foreman starts whatever is missing; the setup checklist on the Dashboard
- [Project page](./features/project-page.md): one team at most; its sections in the sidebar
  - [Sessions](./features/sessions.md) (Session): the terminal multiplexer
  - Inbox: [Reviews](./features/reviews.md#the-inbox), then [Needs you](./features/project-pipeline.md#1-needs-you)
  - [Wiki reader](./features/wiki-reader.md) (Wiki)
  - [Board](./features/board.md)
  - [Repos](./features/repos.md)
  - [Project team](./features/project-team.md) (Team: agents, activity, in flight, merged)
  - [Project pipeline](./features/project-pipeline.md)
  - [Project activity](./features/project-activity.md) (Team → Activity)
  - [Project settings](./features/project-settings.md) (Settings)
  - [Add a project](./features/add-project.md)
- Teams and agents
  - [Team page](./features/team-page.md)
  - [Agent page](./features/agent-page.md)
  - [Deploy a team](./features/deploy-team.md)
- [Chief of Staff](./features/chief-of-staff.md)
- [Requests](./features/requests.md): the five kinds of things that need the CEO
- [Pipeline and merges](./features/pipeline-and-merges.md)
- [Reviews](./features/reviews.md): a branch's change file by file, comments to the lead, the merge
- [Notifications](./features/notifications.md)
- [Settings screen](./features/settings.md)

## Architecture

- [Building blocks](./architecture/building-blocks.md): the blocks, their interfaces, and what is proven
- [Protocol](./architecture/protocol.md): the messaging and workflow schema: who sends what, as which struct, with which lifecycle
- [Overview](./architecture/overview.md): modularity rules, process model, non-functional requirements
- [Tech stack](./architecture/tech-stack.md): languages, libraries, repo layout, build and test
- [Agent runtime](./architecture/agent-runtime.md): instance lifecycle, concurrency, recovery
- [Session runner](./architecture/session-runner.md): where harness processes run (Podman, PTY)
- [Terminal state](./architecture/terminal-state.md): server-side screens, snapshots, terminal queries
- [Harness adapters](./architecture/harness-adapters.md)
- [Sandboxes and git policy](./architecture/sandboxes-and-git.md)
- [Team tools](./architecture/team-tools.md): the agent tools and message delivery
- [Session prompt](./architecture/session-prompt.md)
- [Wiki engine](./architecture/wiki-engine.md)
- [Persistence](./architecture/persistence.md): state.db, logs, file watching
- [API](./architecture/api.md): HTTP, SSE and WebSocket surface
- [Security](./architecture/security.md): threat model

## Design

- [Design system](./design/design-system.md): tokens, type, components
- The clickable UI prototype: [`design/prototype/`](./design/prototype/foreman-prototype.dc.html) (open in a browser; reference only, not production code)
