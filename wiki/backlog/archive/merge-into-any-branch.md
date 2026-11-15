---
type: task
status: done
priority: high
tags: [feature]
epic: git
---

# Merges into any branch of the CEO's, signed as the CEO

**Asked (the CEO, 2026-11-06):** a default branch is a fine hint for the lead, but only being able to merge into it, or into a target the lead made, is not. Any branch can be where a merge goes, and merges are the CEO's: made with the CEO's git identity and signature. A feature branch the CEO pushes is the CEO's to approve or reject; an agent that needs a branch uses its own. GitHub rejects the CEO's unsigned commits, so what lands on the CEO's branches must be signed as the CEO's config says, and nothing an agent does is.

Decided ([Pipeline and merges → Where a merge goes](../../product/features/pipeline-and-merges.md#where-a-merge-goes)):
- the list of targets is gone: a merge request goes into any branch of the repo but an agent's (`wt/*`, `lead/*`), the default when the lead names none; the CEO can move it to any such branch on the review
- agents start work from anywhere (any branch, a remote's, a commit) and merge whatever they like into their own branches: nothing gained by refusing, and the review shows what a branch brings along
- the rules about one card or topic per branch and at most two open merge requests become advice: more when the work justifies them, and work held back is said
- Merge is local only (nothing is pushed); Create PR is a later card (The forge card (retired 2026-11-15, [Remove the forge](./remove-forge.md)))

- [x] store: migration 021 keeps each repo's default branch (`project_repos.default_branch`) and drops `repo_targets`
- [x] home, server, UI: `CreateTarget`, `RemoveTarget`, `ForgetGoneTargets`, the targets route, the Repos chips and `targetsOf` removed; a default branch is any branch but an agent's
- [x] workflow: `create_target` and `targets()` removed; `request_merge` into any branch but an agent's (`agent_branch`, `no_such_branch`); `switch` from any branch or commit (`no_such_ref`); what has landed (Clean up, the resume notice, a hand-off with nothing new) is what a branch of the CEO's holds
- [x] git: no blanket `commit.gpgsign=false`; the CEO's commits (Merge's squash with `commit-tree -S`, Foreman's saves) follow the CEO's config, agents' never sign; `HeldBy`; `Switch` from any commit; `CreateBranch` removed
- [x] prompt: the tools and the softened advice
- [x] tests; specs
