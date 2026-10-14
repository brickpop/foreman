---
type: task
status: in-progress
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Worktrees per instance: the Git block's `Prepare`

**Why:** every agent works in its own git worktree of each code repo, on its own branch, so agents never collide and no agent touches the CEO's checkout ([Sandboxes and git policy](../product/architecture/sandboxes-and-git.md), [Building blocks → Git](../product/architecture/building-blocks.md#8-git-git)).

Blocked by [Workspace and config](./workspace-config-events.md) (the project's code repos).

**Done when:** `Git.Prepare(instance, repo)` makes `<project>/.sandboxes/<agent>/<repo>/` with `git worktree add` on the host if it doesn't exist (members on `wt/<agent>/main`, the lead on `lead/<team>/integration`, both created from `main` when missing) and reuses it otherwise; `.sandboxes/` is in `.git/info/exclude`; the [Instance manager](./instance-manager.md) mounts each worktree (rw) and each repo's `.git` (**read-only**) at their host paths and the live project repo at `<sandbox>/project` (rw for the lead, ro for the others; its `.git` ro for everyone); inside the container git reads work (`status`, `diff`, `log`, `blame`) and every write fails, on macOS and Fedora. Commits come later, through the `commit` team tool ([Messaging and hand-offs](./epics/messaging-handoffs.md)); decided: in epic 1 agents can't commit, and the CEO looks at the uncommitted work in each worktree.

- [x] `Prepare(repo, worktree, branch, base)` in `internal/git`, one git command at a time per repo; tests on real temporary repos: new, existing (left on its branch with its work), an existing branch whose folder was deleted, a branch checked out elsewhere, a repo without `main` (the error points at `main` in project.toml), an empty repo, another repo in the way, twelve at once
- [x] a check that git works in a container through the same-path, read-only `.git` mount, on both machines: `git status`, `diff` and `log` succeed (`git status` normally refreshes the worktree's index under `.git/worktrees/<name>/` and must skip that quietly), and `git commit` and `git checkout -b` fail cleanly: `TestGitThroughAReadOnlyGitMount` in `just test-podman` (composing these mounts for each instance is the [Instance manager](./instance-manager.md)'s)
- [x] `.git/info/exclude` entry: `Exclude(path)` adds `/<path>/` to the enclosing repo's exclude file, once; nothing outside a repo
- [ ] `just test-podman` on the CEO's Mac: git inside the container through the read-only `.git`
- [x] specs: [Sandboxes and git policy](../product/architecture/sandboxes-and-git.md) and [Session runner](../product/architecture/session-runner.md) are `partial`
