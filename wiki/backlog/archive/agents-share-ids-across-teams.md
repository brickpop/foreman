---
type: task
status: done
priority: medium
tags: [feature, ui]
epic: workflow
---

# Agents share ids and names across teams

**Why:** agent ids and display names had to be unique across every team, so a second team couldn't have its own `QA` or `Auditor`. A project has one team at a time, so the rule only added friction: "I would like all team members to be able to have their role as the name" (the CEO, 2026-11-02).

## Decided (the CEO, 2026-11-02)

- Ids and names are **unique within the team only**; no suffixes. Two teams' Auditors are "the same role in spirit".
- Anything naming an agent finds it **through its deployment's team** (`team@project`), never by the id alone.
- A later team's agent of the same id on the same project **takes over** the earlier one's worktree, `wt/<agent>/…` branches, harness folder and logs, but **starts its conversation fresh** ([Team definition → Schema](../../product/models/team-definition.md#schema)).
- The Chief of Staff no longer asks for names unique across teams, nor lists the other teams' members.

## Done

- [x] `crossTeamProblems` gone (load and save); `TeamConfig.validate` refuses a display name twice in a team (compared trimmed)
- [x] The Chief of Staff's prompt: unique in the team, a role is a fine name; only team ids listed as taken
- [x] UI: `agentOf(teams, deployment, instance)`; `namesOf` takes the deployment, so an older session names its own team's agents; branches and card assignees through the project's deployment (`deploymentOn`: the running one, else the one stopped last); `lastTeam` replaced by `latestStopped`
- [x] Tests: two teams sharing ids load and check clean, a name twice in a team is refused; `agentOf`, `branchOwner`, `deploymentOn`, a notice naming the right team's agent
- [x] Specs: Team definition, Agent soul, Instance, Chief of Staff, API, Sandboxes and git, Agent runtime, Repos, Project deployment, Project activity

## Left

- A redeploy that continues conversations must not continue another team's: noted on [Continue conversations on redeploy](./continue-conversations-on-redeploy.md). Today every new deployment starts fresh, so nothing leaks yet.
