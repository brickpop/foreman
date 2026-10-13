# Product

Foreman's product definition. Every entry carries a `status` (`planned` / `partial` / `built`); see `WORKFLOW.md` at the project root. Open questions live in each entry's `## Open questions` section.

- [Vision](./vision.md): summary, goals, non-goals, roles, non-functional requirements
- [Glossary](./glossary.md)
- [Use cases](./use-cases.md): the end-to-end scenarios v1 must pass

## Data models

Files Foreman reads or owns, and the runtime state it keeps.

- [Workspace](./models/workspace.md): folder layout and the config / state / content split
- [Project config](./models/project-config.md) (`project.yaml`)
- [Team definition](./models/team-definition.md) (`.foreman/teams/<team>.yaml`)
- [Agent soul](./models/agent-soul.md)
- [Workspace settings](./models/workspace-settings.md) (`settings.yaml`)
- [Secrets](./models/secrets.md)
- Runtime state (`state.db`)
  - [Deployment](./models/deployment.md)
  - [Instance](./models/instance.md)
  - [Request](./models/request.md)
  - [ACP message](./models/acp-message.md)
  - [Pipeline item](./models/pipeline-item.md)
  - [UI state](./models/ui-state.md): read markers and UI preferences

## Features

- [App shell](./features/app-shell.md): sidebar, toasts, drawers, dialogs
- [Home](./features/home.md)
- [Sessions](./features/sessions.md): the terminal multiplexer
- Project
  - [Wiki reader](./features/wiki-reader.md) (Overview tab)
  - [Changes](./features/changes.md)
  - [Board](./features/board.md)
  - [Project settings](./features/project-settings.md)
  - [Add a project](./features/add-project.md)
- Teams and agents
  - [Team page](./features/team-page.md)
  - [Agent page](./features/agent-page.md)
  - [Deploy a team](./features/deploy-team.md)
- [Chief of Staff](./features/chief-of-staff.md)
- [Requests](./features/requests.md): the five kinds of things that need the CEO
- [Pipeline and pull requests](./features/pipeline-and-prs.md)
- [Notifications](./features/notifications.md)
- [Workspace settings screen](./features/settings.md)

## Architecture

- [Overview](./architecture/overview.md): components and how they connect
- [Tech stack](./architecture/tech-stack.md): languages, libraries, repo layout, build and test
- [Agent runtime](./architecture/agent-runtime.md): instance lifecycle, concurrency, recovery
- [Session runner](./architecture/session-runner.md): where harness processes run (Podman, PTY)
- [Harness adapters](./architecture/harness-adapters.md)
- [Sandboxes and git policy](./architecture/sandboxes-and-git.md)
- [ACP](./architecture/acp.md): the agent tools and message delivery
- [Session prompt](./architecture/session-prompt.md)
- [Forge adapters](./architecture/forge-adapters.md)
- [Wiki engine](./architecture/wiki-engine.md)
- [Persistence](./architecture/persistence.md): state.db, logs, file watching
- [API](./architecture/api.md): HTTP, SSE and WebSocket surface
- [Security](./architecture/security.md): threat model

## Design

- [Design system](./design/design-system.md): tokens, type, components
- [Avatar](./design/avatar.md)
- The clickable UI prototype: [`design/prototype/`](./design/prototype/foreman-prototype.dc.html) (open in a browser; reference only, not production code)
