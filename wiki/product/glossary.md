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
- **Lead branch**: `lead/*` (e.g. `lead/integration`), where the lead integrates handoffs. It is the only source of PRs to `main`.
- **Harness**: the CLI program that runs an agent (Claude Code, Codex CLI, Pi, Hermes Agent, or a custom command). See [Harness adapters](./architecture/harness-adapters.md).
- **Session runner**: where a harness process runs and how Foreman attaches to its terminal (a Podman container in v1). See [Session runner](./architecture/session-runner.md).
- **ACP**: the agent communication protocol Foreman exposes to harnesses (as MCP tools or similar). It provides `ask`, `reply`, `handoff`, `escalate` and `open_pr`. See [ACP](./architecture/acp.md).
- **Request**: something that needs the CEO, of one of five kinds: approval, question, review, finding, blocked. See [Requests](./features/requests.md).
- **Playbook**: the team's written workflow (who does what, when, and when to escalate). It is part of the team definition.
- **Gates**: project rules that always escalate to the CEO (e.g. "new dependency"). They are part of project config (`always_ask_ceo`).
- **Forge**: where PRs live: GitHub, Forgejo, Gitea, or `none` (local-only PRs inside Foreman). See [Forge adapters](./architecture/forge-adapters.md).
- **Pipeline item**: one unit of work tracked from worktree to PR. See [Pipeline item](./models/pipeline-item.md) and [Pipeline and pull requests](./features/pipeline-and-prs.md).
- **Chief of Staff (CoS)**: the built-in, Foreman-owned agent session that handles setup and anything the UI doesn't cover. See [Chief of Staff](./features/chief-of-staff.md).
- **CEO**: the single human user of Foreman.
