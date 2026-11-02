---
type: task
status: done
priority: medium
tags: [feature]
epic: git
---

# Merge targets

**Why:** every merge request lands on its repo's one main branch, and every branch starts from it. The CEO wants work to land on other branches too ("If I ask the agent that 2 separate agents do different things and each lands in a separate target branch"), the lead creating them from any commit, and merging into any of them approved by the CEO only (the CEO, 2026-11-02).

The design: [Pipeline and merges → Merge targets](../../product/features/pipeline-and-merges.md#merge-targets).

## Decided (the CEO, 2026-11-02)

- Targets are per code repo: its main branch (the default) and any others. Only the CEO's merges write them; anything into a target is approved by the CEO.
- The **lead** creates new ones, `create_target(repo, name, from)`, from any commit or branch of that repo; they are targets at once, no approval (no code changes). One call per repo.
- The target belongs to the piece of work, not to an agent or a team: `switch(…, from)` starts it there, `request_merge(…, into)` lands it there.
- The CEO sees the targets in Repos, removes them there, and can change a review's target before deciding.

## Steps

- [x] Rename "main branch" to "default branch" everywhere; retire `merge_to_main`
- [x] Records: a repo's targets (name, made by, from, when), the main branch first; a merge request's `target` (existing ones: their repo's main branch at the migration)
- [x] `create_target` (lead only) with its refusals; `switch` with `from`; `request_merge` with `into`
- [x] Merge to `<target>`, conflicts, "what counts as merged" and the decided diff, all against the merge request's target; "when a target moves" notices per target. The forge PR's base is carried by [The forge](../git-forge.md), not built yet
- [x] The review: `→ <target>`, its button, changing the target (diff and conflicts redone)
- [x] Repos: the targets list, Remove (refused while one is open into it), missing ones flagged
- [x] Session prompt: each repo's targets
- [x] Tests: each refusal; a branch from a target merges into it; a review retargeted shows the new diff; removing a target with an open merge request is refused; a target deleted outside is flagged; notices go only to that target's leads
- [x] Specs: Pipeline and merges (the section loses "Planned"), Merge request, Team tools, Protocol, Repos, Reviews, Project config, Session prompt

## Edges settled (the CEO, 2026-11-02)

- A review spans every repo with new commits: an `into` that one of them lacks is refused, naming those repos; no fallback.
- A target name that exists in the repo, whoever made it: refused.
- Idle desks follow the main branch only (the repo's default target); other targets have no idle desks.
- **"Main branch" becomes "default branch"**, all the way: UI, specs, prompts, tool texts, and the names in code, API and database (`Main`→`Default`, JSON `main`→`defaultBranch`, the column `main`→`default_branch`). Branch names (`wt/<agent>/main`) and `merge("main")` stay.
- **`merge_to_main` is retired** from project.toml: it only ever took `"ceo"`. Old files load; the key is ignored with a note to remove the line, as the old `main` key.

## Built (2026-11-02)

Migration 014 (`project_repos.default_branch`, `repo_targets`, `merge_requests.target`). Decided while building:

- A refresh of a merge request without `into` keeps its current target (the CEO may have moved it).
- Moving a review's repo to another target forgets a conflict found before and tells the lead.
- Work any target holds counts as merged, for hand-off's `nothing_new` and the resume notice.
- A dismissed branch still checked out goes back to the default branch, as idle desks do.
- Revised the same day, after the CEO's review (migration 015):
  - **One list of targets** per repo, the default flagged among them. Changing the default moves the flag: the old one stays a target, merge requests keep theirs.
  - **`from` must be held by a target**: a target starts only from work the CEO merged, so creating one changes no code and needs no approval.
  - **Targets are never in the prompt**; every agent has `targets()` instead, so a target made or removed leaves prompts as they were.
  - **Git says whether a target exists**: one whose branch went is forgotten within 30 s, unless a review goes into it, which then says so and can't merge that repo until moved.
  - **A refresh naming `into` moves every repo the review has open**, or is refused: a review never splits between targets unasked.
  - **Comments on removed lines are outdated after a retarget**, as on a forge when a PR's base changes (each comment keeps the target it was read against).
  - **A merge request moved into a target that holds it already is merged at once**, and the tool says so, rather than showing open until the next 30 s check.
  - **An agent's branch (`wt/`, `lead/`) is never a default or a target.**
- Also (2026-11-02): **a project with no code repo can be deployed** (a research team's): its agents get no git tools, and the lead commits the project repo ([Team tools → No code repo](../../product/architecture/team-tools.md#no-code-repo)). The setup checklist no longer asks for a code repo.
