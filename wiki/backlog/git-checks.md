---
type: task
status: todo
priority: low
tags: [feature]
epic: git
---

# Run the project's checks on a clean checkout

**Parked (the CEO, 2026-10-17): a later feature, if at all.** Open when it comes back: where one `checks` command runs in a project of several repos (per repo, or once over all of them), whether a merge request waits for its checks, whether a hand-off is held until they pass, and a timeout. Until then the agents run the checks themselves: the session prompt says "Before any handoff, run: <checks>".

**Why:** `checks` in `project.toml` (e.g. `go test ./...`) should run before work moves on: when a member hands off, and when the lead asks to merge ([Sandboxes and git policy → Checks](../product/architecture/sandboxes-and-git.md)).

**Done when:** a hand-off runs the checks on the work item's branch, and `request_merge` on the merge request's, each in a throwaway container from the project's image on a clean checkout of the branch; the result lands in `checks_status` and shows in Changes; a project without `checks` skips them; tests.

- [ ] the throwaway run (engine, clean checkout, timeout, output kept)
- [ ] on hand-off and on `request_merge`; results in the work item and the merge request
- [ ] Changes shows them; tests; specs
