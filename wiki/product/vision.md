---
type: spec
status: planned
tags: [architecture, home]
---

# Vision

Foreman is a self-hosted server and web UI. It lets one person (the **CEO**) run several **teams of AI coding agents** across several **projects** on their own machine.

- Foreman keeps everything of its own in one folder, its **[home](./models/home.md)** (`~/.foreman`). A **[project](./models/project-config.md)** is a record there plus a [project repo](./models/project-config.md#the-project-repo) (its rules and its wiki).
- A project names one or more **git repositories**, wherever the CEO keeps them, and has a **wiki** (a folder of Markdown files that acts as the project's shared memory, see [Wiki engine](./architecture/wiki-engine.md)).
- A **[team](./models/team-definition.md)** is a reusable definition: a roster of [agents](./models/agent-soul.md), each with a personality, skills and a job description, plus a playbook for how they work together.
- A team is **[deployed](./models/deployment.md)** onto a project, which has one team at most and keeps its goal in its wiki. One team can be deployed on many projects at the same time. Each deployment runs its own sessions.
- Each agent in a deployment runs as a real **coding harness process** (Claude Code, Codex CLI, Pi, Hermes Agent or a custom command), an [instance](./models/instance.md). It works in its **own git worktree** of each of the project's repos, on its own branch ([Sandboxes and git policy](./architecture/sandboxes-and-git.md)).
- Agents talk to each other through a message router ([Team tools](./architecture/team-tools.md)). Only the designated output of a request crosses from one sandbox to another.
- Only the team **lead** can ask for changes to reach a repo's default branch (or another of the CEO's branches, a release branch say), with a **merge request**. The CEO reviews it in Foreman, file by file, with comments back to the lead, and decides: **Merge** (Foreman merges locally), **Request changes**, or **Dismiss** with a reason. Foreman never merges on its own, and never pushes code: the CEO does, with their own git ([Pipeline and merges](./features/pipeline-and-merges.md)).
- Anything that needs the CEO arrives as a typed **[request](./features/requests.md)** (approval, question, review, finding, blocked). It shows in the UI and as a browser [notification](./features/notifications.md).
- The UI is also a **terminal multiplexer** ([Sessions](./features/sessions.md)). Every agent session's stdin/stdout/stderr is live in the browser, with copy and paste.

Terms are defined in the [Glossary](./glossary.md). The end-to-end scenarios v1 must pass are the [Use cases](./use-cases.md).

## Goals

1. Run several agent teams in parallel on local code, with clear roles and a clear path to each repo's default branch.
2. Make the CEO's job small: see what needs a decision, decide, move on.
3. Keep agents isolated (separate worktrees and containers) while letting them collaborate (through [team tools](./architecture/team-tools.md)).
4. Reuse teams and agent "souls" across projects without copying them.
5. Keep project knowledge in plain Markdown, in the project, maintained by agents.
6. Stay harness-agnostic: any CLI coding agent that runs in a terminal can be a team member ([Harness adapters](./architecture/harness-adapters.md)).

## Non-goals (v1)

- Multi-user accounts, permissions or remote multi-tenant hosting.
- Cloning code repositories. The CEO keeps them where they like and adds them to a project by path.
- Editing project config or wiki prose from the UI. Files are edited in the CEO's editor or by agents.
- Talking to a forge (GitHub, Forgejo, Gitea): no PRs, no tokens (the CEO, 2026-11-15). The CEO pushes and opens PRs with their own tools.
- Billing, model routing or token accounting, beyond showing harness rate-limit failures.

## Users and roles

| Role | Who | What they can do |
|---|---|---|
| **CEO** | The human operator, sole user | Everything. Answers requests, decides merge requests (merge, create a PR, or dismiss), deploys teams, talks to any agent. |
| **Chief of Staff** | A built-in agent (a harness session owned by Foreman, see [Chief of Staff](./features/chief-of-staff.md)) | Interviews the CEO to define roles and teams, and proposes the team files, which Foreman writes once the CEO confirms them. |
| **Team lead** | One agent per team | Breaks down goals, delegates, reviews handoffs, integrates on `lead/<project>/<team>/*` branches, requests merges into the CEO's branches. The CEO's default point of contact for the team. |
| **Members** | Engineer, Architect, QA, Reviewer, custom | Work in their own worktree branches (`wt/<project>/<agent>/<card>`) and hand off to the lead. Cannot write the CEO's branches or `lead/<project>/<team>/*`. |

Roles with default job descriptions (used by the Chief of Staff and the example teams): Team Lead, Architect, Engineer, QA / Tester, Reviewer ([Agent soul](./models/agent-soul.md)).

## Decided

- **Server in Go**, one binary serving the API and the built UI ([Tech stack](./architecture/tech-stack.md)).
- **Every agent instance runs in a Podman container from day one.** Isolation is enforced by what is mounted into the container (its own worktrees, the project's knowledge base read-only or read-write), not by convention ([Session runner](./architecture/session-runner.md), [Security](./architecture/security.md)).
- **Where sessions run is a swappable interface** (the session runner), so other runners (a plain PTY for tests, tmux) can be added without touching the rest.
- **Specs carry a `status`** (`planned` / `partial` / `built`); see `WORKFLOW.md` at the project root.

## Non-functional requirements

- **Local first**: binds to localhost by default. There is no telemetry. Works offline, except harnesses that need the network.
- **Security**: there are no user accounts in v1, but every request needs the per-install access token from the URL Foreman prints at start ([Security](./architecture/security.md#access-token)); binding to the network shows a warning. Secrets are never written to a repo or to config; Foreman redacts the secrets it knows from session logs, and anything else a harness prints is a documented limit. See [Security](./architecture/security.md).
- **Isolation**: a separate worktree and container per instance. Team tools return only designated outputs. Write access to the knowledge base is limited to the lead; the others use `contribute`.
- **Reliability**:
  - Instances survive UI disconnects.
  - On server restart, deployments are restored from `state.db` and sessions are restarted with the last task note ([Agent runtime](./architecture/agent-runtime.md)).
  - Session logs keep each session's full history ([Persistence](./architecture/persistence.md)); a reload shows the exact current screen from a snapshot ([Terminal state](./architecture/terminal-state.md)).
- **Performance**: 20+ concurrent sessions on a laptop. Terminal latency under 50ms locally. File changes reflected in the UI within 1s.
- **Observability**: per-instance logs, a team-message log, a request history, and a server log with harness exit codes.
- **Portability**: macOS and Linux first. Windows via WSL.

## Out of scope for v1 / future

- Multi-user, auth, roles beyond CEO.
- Cloning or provisioning repos from the UI.
- An in-app diff viewer and inline review comments.
- Wiki graph view, table views and "read later" (wikiview has graph and read later; they can be adopted later).
- Cost and token budgets per deployment.
- Scheduling (cron-style recurring goals), standup automation beyond what a member's job description asks for.
- Cross-deployment messaging (deliberately disallowed).
- Mobile layout beyond basic responsiveness.
