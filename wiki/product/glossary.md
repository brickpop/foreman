---
type: spec
status: planned
tags: [architecture, workspace]
---

# Glossary

- **Workspace**: the root folder the server runs on (e.g. `~/agents-ws`). It contains projects and Foreman's own data in `~/agents-ws/.foreman/`. See [Workspace](./models/workspace.md).
- **Project**: a subfolder of the workspace containing `.foreman/project.yaml`. It holds git repos, a `wiki/` and agent sandboxes. See [Project config](./models/project-config.md).
- **Repository**: a git repo inside a project folder. Foreman detects repos; it never clones them.
- **Wiki bundle**: `<project>/wiki/`, a folder of Markdown files with YAML frontmatter, plus `wiki.toml`. Compatible with the agentic-wiki / wikiview format. See [Wiki engine](./architecture/wiki-engine.md).
- **Board**: a kanban view over wiki entries, declared in `wiki.toml` under `[[tool.wikiview.board]]`. See [Board](./features/board.md).
- **Team definition**: a reusable roster and playbook, stored in `.foreman/teams/<team>.yaml`. See [Team definition](./models/team-definition.md).
- **Agent (soul)**: one member of a team definition: name, role, personality, skills, job description, harness and look (avatar). One soul can run in many deployments. See [Agent soul](./models/agent-soul.md).
- **Deployment**: a team assigned to a project with a goal. This is runtime state, not config. See [Deployment](./models/deployment.md).
- **Instance / session**: one agent running in one deployment. It is a live harness process with a PTY. Identified as `agent@project` (e.g. `mara@ledger`). See [Instance](./models/instance.md).
- **Sandbox**: an agent instance's private clone of the project repos: `<project>/.sandboxes/<agent>/`. See [Sandboxes and git policy](./architecture/sandboxes-and-git.md).
- **Worktree branch**: `wt/<agent>/<card>`, where members do their work.
- **Lead branch**: `lead/<team>/<topic>` (the default `lead/<team>/integration` is created at deploy), where the lead integrates handoffs; a lead can have any number. Lead branches are the only source of PRs to `main`, one open PR per branch at a time.
- **Harness**: the CLI program that runs an agent (Claude Code, Codex CLI, Pi, Hermes Agent, or a custom command). See [Harness adapters](./architecture/harness-adapters.md).
- **Session runner**: where a harness process runs and how Foreman attaches to its terminal (a Podman container in v1). See [Session runner](./architecture/session-runner.md).
- **Team tools**: the MCP tools Foreman exposes to every harness: `ask`, `reply`, `handoff`, `escalate` and `open_pr`; agent-to-agent traffic through them is **team messages**. Called "ACP" in the original PRD. See [Team tools](./architecture/team-tools.md).
- **Session driver**: how Foreman drives a harness session: through its TUI in a PTY (v1), or later through the Agent Client Protocol. See [Harness adapters](./architecture/harness-adapters.md#session-drivers).
- **Agent Client Protocol (ACP)**: Zed's open protocol for driving coding agents over JSON-RPC (prompts in; messages, tool calls and permission requests out). A future session driver; not the team tools. See [Harness adapters](./architecture/harness-adapters.md#session-drivers).
- **Request**: something that needs the CEO, of one of five kinds: approval, question, review, finding, blocked. See [Requests](./features/requests.md).
- **Playbook**: the team's written workflow (who does what, when, and when to escalate). It is part of the team definition.
- **Gates**: project rules that always escalate to the CEO (e.g. "new dependency"). They are part of project config (`always_ask_ceo`).
- **Forge**: where PRs live: GitHub, Forgejo or Gitea. The CEO reviews and merges PRs there; Foreman never merges. A project with `forge: none` has no PRs: the lead branch is published into the project repo for the CEO to merge with git. See [Forge adapters](./architecture/forge-adapters.md).
- **Card** (board card): a wiki task entry shown on a [board](./features/board.md). "Card" means only this; members work on one card per worktree branch (`wt/<agent>/<card>`).
- **Work item**: one agent's branch on its way into the lead branch (in worktree → handed to lead → lead reviewing → integrated). Informational: it never needs the CEO. See [Work item](./models/work-item.md).
- **Pull request (PR)**: the lead's proposal to merge the lead branch into `main`, carrying the integrated work items; the only pipeline entry that raises a request (the review). See [Pull request](./models/pull-request.md) and [Pipeline and pull requests](./features/pipeline-and-prs.md).
- **Request panel**: the UI component that shows one [request](./models/request.md) on Home and in Changes.
- **Chief of Staff (CoS)**: the built-in, Foreman-owned agent session that handles setup and anything the UI doesn't cover. See [Chief of Staff](./features/chief-of-staff.md).
- **CEO**: the single human user of Foreman.
