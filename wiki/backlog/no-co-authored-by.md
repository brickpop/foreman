---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: git
---

# Commit messages without `Co-Authored-By`

**Why:** the CEO wants a commit to carry its message and nothing else, with no `Co-Authored-By` trailers (the CEO, 2026-10-29).

## Where they came from

- **The squash merge to `main`.** Foreman added them itself, one per author of the branch's commits ([Pipeline and merges](../product/features/pipeline-and-merges.md), "Always a squash").
- **The agents' own commits** (the `commit()` tool, on their branches). Claude Code asks its model to end commit messages with a `Co-Authored-By: Claude …` trailer unless its `attribution` setting is emptied. These commits never reach `main`, but they show on the branches and on a forge's pull request.

## Decided (the CEO, 2026-10-29)

- Neither Foreman nor the harness adds them.
- `commit()` also drops any that come anyway (a harness ignoring the setting, or another harness), "if present".

**Lost:** `main` no longer says which agents wrote a change. The merge request's wiki entry and the branch's history still do.

## Done (2026-10-29)

- [x] Squash: the message alone. `coAuthors` and the author lookup it needed are removed (`internal/git/merge.go`).
- [x] `commit()`: `withoutCoAuthors` drops every line starting `Co-Authored-By:`, in any case, then trailing blank lines. A message of nothing else is refused as `empty` (`internal/workflow/workflow.go`).
- [x] Claude Code: `"attribution": {"commit": "", "pr": ""}` in each instance's `settings.json` (`internal/harness/claudecode/claudecode.go`).
- [x] Tests:
  - `internal/git/merge_test.go`: the title alone, even with several authors.
  - `internal/workflow/workflow_test.go`: the squash; `TestCommitDropsCoAuthoredByLines` (any case, indented, mid-message, other trailers kept, a message that only mentions it kept, only trailers is `empty`).
  - `claudecode_test.go`: the setting is written.
- [x] Specs: [Pipeline and merges](../product/features/pipeline-and-merges.md), [Protocol → `commit`](../product/architecture/protocol.md#commit), [Session runner](../product/architecture/session-runner.md).
- [ ] Seen on a real Claude Code (`just test-claude`): that the `attribution` key is the one this Claude Code version reads.
