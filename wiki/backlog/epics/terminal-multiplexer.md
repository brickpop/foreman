---
type: epic
status: todo
order: 1
---

# Terminal multiplexer

**Goal:** a working multiplexer of isolated agents. Projects (their wiki, board and definition) get teams (souls and skills, as team files), a deployment starts one agent per member in its own Podman container, and the CEO watches and talks to each of them in the browser. No messaging between agents yet: the CEO talks to every agent directly.

**Usable when done:** start Foreman, prepare images, log in to Claude Code, add a project (creating its project repo), read its wiki and board, edit a team's souls, deploy the team with a goal (each agent in its own worktree), and work with each agent in the project's Session (tabs, split, reload with an exact snapshot); stop and restart Foreman without losing the agents.

**Journey steps:** 1–5, 7–10, 14, 19, 27–29 of [the journey](../../product/journey.md). Step 6 (creating a team) is done by editing the team file by hand until the Chief of Staff (epic 4).

**How agents get the project's code (decided):** every agent gets its own **git worktree** of each code repo, at `~/.foreman/sandboxes/<project>/<agent>/<repo>/`, on its own branch (`wt/<agent>/main` for members, `lead/<team>/integration` for the lead), so agents never collide in one working tree and no agent touches the CEO's checkout ([Sandboxes and git policy](../../product/architecture/sandboxes-and-git.md)). Each repo's `.git` is mounted with it, **read-only**: agents read git (`status`, `diff`, `log`), and every git write is a team tool Foreman runs on the host ([Agents and git](../../product/architecture/sandboxes-and-git.md#agents-and-git)). Those tools (`switch`, `commit`, `merge`) need the agent link, so they arrive in [Messaging and hand-offs](./messaging-handoffs.md). **Decided: in this epic agents can't commit.** The CEO drives each session by hand and looks at the uncommitted work in each worktree. The project's knowledge base is the [project repo](../../product/models/project-config.md#the-project-repo) `project/`: one live checkout mounted into every container, read-write for the lead, read-only for the others.

The global repo `~/.foreman/config/` is a git repo (team files and settings, one commit per UI save), and all config files are TOML ([Home](../../product/models/home.md)).

In this epic there is no messaging yet: every agent works only on what the CEO types into it.

**Blocked by:** [Prove](./prove.md) (experiments A, B and E), and [the decisions on the journey's gaps](../archive/decide-journey-gaps.md).

**Exit criteria:** the steps above work end to end on macOS and Fedora with Claude Code, covered by tests (Go and UI) and by a written manual check on both machines; the specs they touch are `built` or `partial` with the gaps named.
