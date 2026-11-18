---
type: task
status: todo
priority: high
tags: [feature, decision]
epic: git
---

# Submodules, everywhere Foreman touches a repo

**Why:** the CEO, 2026-11-16: "git submodules needs a card to work properly, in general". Deleting a branch checked out by a stray worktree failed on a repo with submodules (`git worktree remove` refuses any worktree with one checked out, however clean). That was fixed, but it showed that every git operation Foreman added after [Submodules in worktrees](./submodules-in-worktrees.md) was written and tested without submodules. The CEO's repos (Foundry projects, `lib/*`; Aragon's) mostly have them.

What works today: an agent's worktree gets its submodules at every start and on `open`/`switch`/`merge` (`SyncSubmodules`, from the CEO's checkout when private, [Private submodules](./private-submodules.md)); `set_submodule`; the refusals on commit, switch and merge; the links mended after a move (`relinkSubmodules`); removing a clean worktree with submodules (forced after `Clean`, 2026-11-16).

## Known gaps

1. **Foreman's clones have no submodules** ([Portable projects](./portable-projects.md), part 3). `git.Clone` doesn't recurse, so a repo cloned into `repos/` (Repos → Add, **Clone it**, Bring here) has none checked out: the CEO's own build there fails. A private one (SSH) can't reach the agents either: an agent's submodule comes from the CEO's checkout (`<repo>/.git/modules/<name>`), which a Foreman clone doesn't have. So on a new machine, Clone breaks exactly the repos [Private submodules](./private-submodules.md) fixed. *To decide:* clone with `--recurse-submodules` with the CEO's own git (the CEO typed the URL, as if they ran the clone themselves; the URLs are the default branch's), or leave it to the CEO (`git submodule update --init` in the clone, said in the toast).
2. **Renaming a repo fails** when an agent's worktree of it has a submodule checked out: `git worktree move` refuses it, with no `--force` for that (`MoveWorktree`, `RenameRepo`). Likely fix: move the folder, `git worktree repair`, then `relinkSubmodules`.
3. **Removing a worktree can lose a submodule commit.** The forced removal checks the worktree clean, submodules included, but a commit made inside a submodule and nowhere else (not pushed, not in the CEO's copy) goes with it. Rare: Foreman only checks submodules out at commits fetched from their URL. Fix: refuse when a submodule's HEAD isn't in its repo's data outside the worktree.
4. **Clean up's kept changes** skip a change inside a submodule: the worktree stays ([Clean up: what its uncommitted option can't keep](./cleanup-kept-changes.md)).
5. **A submodule not filled at a start** is only a `WARN` in the server log: neither the agent nor the CEO is told ([Private submodules](./private-submodules.md), its open step).
6. **URLs:** a URL relative to the repo's remote (`../lib.git`) is refused; nested submodules never come from the CEO's checkout; `set_submodule(url)` takes https only ([Private submodules → edge cases](./private-submodules.md)).
7. **The Repos listing** runs `git status --ignore-submodules=none` per agent worktree, which walks every submodule: the likely cause of the slow `GET /api/projects/:id/repos` the CEO saw on aragon-osx-v1 (2026-11-16, unconfirmed; the calls now run 8 at a time). Fix to weigh: a cheaper "has changes" for that column.
8. **To check, not known broken:** a submodule bump in a review diff and in the file browser (a `commit` entry in a tree); the move script ([One Foreman folder](./foreman-folder.md)) moving worktrees with submodules (`relinkSubmodules` should mend them at the next start); merges and squashes of a branch that moves a pin; `request_merge` checks in a throwaway container needing the submodules.

## Steps

- [ ] decide 1 (recursive clone or not) and fix it, with Bring here and **Clone it**
- [ ] rename a repo whose worktrees have submodules (2)
- [ ] guard the forced removal against a commit only in the worktree's submodule (3)
- [ ] tell the agent and the CEO about a submodule not filled (5, done in [Private submodules](./private-submodules.md))
- [ ] measure the Repos listing on a repo with submodules and fix the cost (7)
- [ ] go through 8, one test each with a submodule, filing what fails
- [ ] a submodule fixture in the git tests that every new worktree or branch operation reuses, so the next one is tested with submodules from the start
- [ ] specs: [Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules) as the one place listing what each operation does with them
