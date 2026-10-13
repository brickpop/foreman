---
type: spec
status: planned
tags: [architecture, workspace]
---

# Vision

Foreman is a self-hosted server and web UI. It lets one person (the **CEO**) run several **teams of AI coding agents** across several **projects** on their own machine.

- A **[workspace](./models/workspace.md)** is a local folder. Each subfolder that Foreman recognises (it holds `.foreman/project.yaml`) is a **project**.
- A project holds one or more **git repositories** and a **wiki** (a folder of Markdown files that acts as the project's shared memory, see [Wiki engine](./architecture/wiki-engine.md)).
- A **[team](./models/team-definition.md)** is a reusable definition: a roster of [agents](./models/agent-soul.md), each with a personality, skills and a job description, plus a playbook for how they work together.
- A team is **[deployed](./models/deployment.md)** onto a project with a goal. One team can be deployed on many projects at the same time. Each deployment runs its own sessions.
- Each agent in a deployment runs as a real **coding harness process** (Claude Code, Codex CLI, Pi, Hermes Agent or a custom command), an [instance](./models/instance.md). It works in its **own clone** of the project's repos ([Sandboxes and git policy](./architecture/sandboxes-and-git.md)).
- Agents talk to each other through a message router ([ACP](./architecture/acp.md)). Only the designated output of a request crosses from one sandbox to another.
- Only the team **lead** can propose changes to `main`, as a pull request on the project's forge (GitHub, Forgejo, Gitea or local-only). The CEO merges or sends it back ([Pipeline and pull requests](./features/pipeline-and-prs.md)).
- Anything that needs the CEO arrives as a typed **[request](./features/requests.md)** (approval, question, review, finding, blocked). It shows in the UI and as a browser [notification](./features/notifications.md).
- The UI is also a **terminal multiplexer** ([Sessions](./features/sessions.md)). Every agent session's stdin/stdout/stderr is live in the browser, with copy and paste.

Terms are defined in the [Glossary](./glossary.md). The end-to-end scenarios v1 must pass are the [Use cases](./use-cases.md).

## Goals

1. Run several agent teams in parallel on local code, with clear roles and a clear path to `main`.
2. Make the CEO's job small: see what needs a decision, decide, move on.
3. Keep agents isolated (separate clones) while letting them collaborate (ACP).
4. Reuse teams and agent "souls" across projects without copying them.
5. Keep project knowledge in plain Markdown, in the project, maintained by agents.
6. Stay harness-agnostic: any CLI coding agent that runs in a terminal can be a team member ([Harness adapters](./architecture/harness-adapters.md)).

## Non-goals (v1)

- Multi-user accounts, permissions or remote multi-tenant hosting.
- Cloning or provisioning repositories. The CEO puts repos in project folders by hand.
- Editing project config or wiki prose from the UI. Files are edited in the CEO's editor or by agents.
- Built-in code review UI (diffs). Review happens on the forge, or locally via `git`.
- Hosting the forge. Foreman connects to an existing GitHub, Forgejo or Gitea.
- Billing, model routing or token accounting, beyond showing harness rate-limit failures.

## Users and roles

| Role | Who | What they can do |
|---|---|---|
| **CEO** | The human operator, sole user | Everything. Answers requests, merges PRs, deploys teams, talks to any agent. |
| **Chief of Staff** | A built-in agent (a harness session owned by Foreman, see [Chief of Staff](./features/chief-of-staff.md)) | Interviews the CEO to define roles and teams, edits config files on a branch, and handles any request the UI doesn't cover. |
| **Team lead** | One agent per team | Breaks down goals, delegates, reviews handoffs, integrates on `lead/*` branches, opens PRs to `main`. The CEO's default point of contact for the team. |
| **Scrum master** | Optional agent role | Owns the project wiki: keeps boards in sync, writes ADRs and standups, pings owners of stale cards. The only role with wiki write access by default. |
| **Members** | Engineer, Architect, QA, Reviewer, custom | Work in their own worktree branches (`wt/<agent>/<card>`) and hand off to the lead. Cannot push to `main` or `lead/*`. |

Default roles available in templates: Team Lead, Architect, Engineer, QA / Tester, Scrum Master, Reviewer ([Agent soul](./models/agent-soul.md)).

## Decided

- **Server in Go**, one binary serving the API and the built UI ([Tech stack](./architecture/tech-stack.md)).
- **Every agent instance runs in a Podman container from day one.** Isolation is enforced by what is mounted into the container (its own sandbox, the wiki read-only or read-write), not by convention ([Session runner](./architecture/session-runner.md), [Security](./architecture/security.md)).
- **Where sessions run is a swappable interface** (the session runner), so other runners (a plain PTY for tests, tmux) can be added without touching the rest.
- **Specs carry a `status`** (`planned` / `partial` / `built`); see `WORKFLOW.md` at the project root.

## Non-functional requirements

- **Local first**: binds to localhost by default. There is no telemetry. Works offline, except harnesses and forges that need the network.
- **Security**: there are no user accounts in v1, but every request needs the per-install access token from the URL Foreman prints at start ([Security](./architecture/security.md#access-token-decided)); binding to the network shows a warning. Secrets are never written to project folders or logs. Agents never receive forge tokens ([Secrets](./models/secrets.md), [Security](./architecture/security.md)). Whether "warning only" is enough, and whether secrets can really be kept out of PTY logs, are open in [Security](./architecture/security.md#open-questions) and [Secrets](./models/secrets.md#open-questions).
- **Isolation**: separate clones per instance. ACP returns only designated outputs. Wiki write access is limited to the scrum master or lead.
- **Reliability**:
  - Instances survive UI disconnects.
  - On server restart, deployments are restored from `state.db` and sessions are restarted with the last task note ([Agent runtime](./architecture/agent-runtime.md)).
  - PTY logs allow scrollback replay ([Persistence](./architecture/persistence.md)).
- **Performance**: 20+ concurrent sessions on a laptop. Terminal latency under 50ms locally. File changes reflected in the UI within 1s.
- **Observability**: per-instance logs, an ACP log, a request history, and a server log with harness exit codes.
- **Portability**: macOS and Linux first. Windows via WSL.

## Out of scope for v1 / future

- Multi-user, auth, roles beyond CEO.
- Cloning or provisioning repos from the UI.
- An in-app diff viewer and inline PR comments.
- Wiki graph view, table views and "read later" (wikiview has graph and read later; they can be adopted later).
- Cost and token budgets per deployment.
- Scheduling (cron-style recurring goals), standup automation beyond the scrum master's own behaviour.
- Cross-deployment messaging (deliberately disallowed).
- Mobile layout beyond basic responsiveness.
