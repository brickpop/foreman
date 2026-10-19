---
type: task
status: backlog
priority: low
tags: [feature]
epic: install-first-run
---

# Profiles per agent

**Why:** the CEO, 2026-10-18. A project uses one [profile](../product/architecture/session-runner.md#credentials) for all its agents (built 2026-10-18). Later: a project allows several profiles, with one as its default, and each agent of a deployment can be set to another of them (the lead on Claude Code, the others on a cheaper Pi model, say).

**To decide when it's scheduled:** where the override is set (Project settings, per agent, rather than the Deploy dialog, which keeps choosing nothing: the CEO, 2026-10-18), whether it is per deployment or per agent of the project, and what a deployment page would need to edit it. It also brings back "Switch profile" on a blocked request (another account or harness for one agent: [Requests](../product/features/requests.md)).

Decide together with [Model, effort and auto-compact per agent](./agent-model-settings.md): a model only makes sense for its harness.
