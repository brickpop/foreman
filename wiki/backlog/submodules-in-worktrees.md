---
type: task
status: in-progress
priority: high
tags: [feature]
epic: git
---

# Submodules in worktrees

**Why:** the CEO will run many Foundry repos, whose dependencies are git submodules under `lib/`. An agent's worktree came without them (`lib/forge-std` empty, `forge build` failing), and the agent can't fill one: a worktree keeps its submodules under `<repo>/.git/worktrees/<name>/modules/`, read-only in the container. Filling each worktree by hand from the CEO's terminal would be chronic work.

**Decided (the CEO, 2026-10-19):** Foreman keeps the submodules in step on the host; agents add or move one with `set_submodule(repo, path, ref, url?)`; nested submodules only with `recursive_submodules = true` in `project.toml`, off by default. The design is in [Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules) and [Protocol → `set_submodule`](../product/architecture/protocol.md#set_submodule).

**Built (2026-10-19), checked (2026-10-20):** every Go test (five runs of the submodule ones), vet, staticcheck and the UI checks pass. The first run found that `set_submodule` to the commit a submodule is already on failed (nothing to commit), though that is how an agent throws away its edits in one: it now discards them and says so, without a commit.

Found while building it (git 2.47, checked by hand before writing the code):
- `submodule update --init` in a worktree writes nothing to the repo's `.git/config` when each URL is given on the command line (`-c submodule.<name>.url=…`); without that, `init` writes the agent-chosen URL into the CEO's config.
- A plain checkout leaves a removed submodule's folder behind (`unable to rmdir`), which `git add --all` commits back. A checkout with `submodule.recurse=true` removes it, but fails outright when the branch pins a commit not fetched yet, so Foreman does a plain checkout, then the sync (update, and removing leftovers).
- `.gitmodules` can say `update = none`, `merge` or `rebase`: the sync forces `checkout`. It can say `ignore = all`, hiding changes from `status`: the checks pass `--ignore-submodules=none`.

- [x] the sync on worktree creation, `switch()`, `merge()` and `abort_merge()`; leftovers removed when unchanged
- [x] refusals: `submodule_changed` (switch, merge, handoff, request_merge), `submodule_conflict` and `nested_repo` (commit)
- [x] `set_submodule`: add, move, settle a conflict during a merge; its refusals; fetches without credentials, https only
- [x] `recursive_submodules`
- [x] integration tests on real repos (git: added, moved, removed, conflicting, nested, a worktree per agent, credentials over a real HTTP git server; workflow: the tool end to end)
- [x] specs: Sandboxes and git policy, Team tools, Protocol, Session prompt, Project config, glossary, building blocks, sessions, API
- [x] run every check in a fresh session
- [ ] the CEO tries a Foundry repo

## Found (2026-10-30): moving a project broke every worktree with submodules

After the CEO moved a project into its workspace, `switch()` failed for every agent: in each `account-bunker` worktree, the four `lib/*` submodules' `.git` files pointed one `../` too high. Moving a project puts each sandbox one folder deeper, and git writes a submodule's links as relative paths: its `.git` file, and its data's `core.worktree` back to it. `git worktree repair` (what the move runs) leaves both alone. `switch()` acts on every repo, so one broken repo blocked all of them.

- [x] `relinkSubmodules` (`internal/git/submodules.go`), run by `Prepare` at every agent start: it rewrites both links of each checked-out submodule, nested ones too, so the worktrees already broken mend at their next start. It runs on the host over a `.gitmodules` the agent can edit, so it only rewrites an existing `.git` file inside the worktree, through no symlink, into the worktree's own git dir.
- [x] tests: a moved worktree (the agents' exact error, then `status`, `--show-toplevel`, `HEAD` and a sync all right), nested submodules, and a `.gitmodules` aimed out of the worktree (a `..` path, a symlink, a `..` name), checked to fail with the guards removed
- [x] spec: [Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules)
- [ ] the CEO restarts the agents of the moved project, and `switch()` works
- [ ] decide with the CEO: renaming a repo whose worktrees have submodules is refused by git (`git worktree move` won't move them). Is the error enough, or should Foreman move them itself (`os.Rename`, then `worktree repair` and this relink)?

## Found (2026-10-30): a private submodule never came

`strategy-doc` pins `platform-doc` as `git@github.com:aragon/platform-doc.git`. Foreman fetches over https only and without the CEO's credentials (agents pick the URLs), so it refused it at every start, switch and merge, and the agent had no way to get it.

**Decided (the CEO, 2026-10-30):** try the https counterpart of an SSH URL, and failing that, take it from the CEO's own checkout.

- [x] `update` goes one submodule at a time: the URL through `httpsOf` (`git@host:path`, `ssh://user@host:port/path` → `https://host/path`), and on failure the CEO's copy, `<repo>/.git/modules/<name>` (`ceoCopy`), with local paths allowed for that one clone only. Agents can already read that copy (the repo's `.git` is mounted in their containers), so nothing new reaches them. Neither works: the error says to run `git submodule update --init <path>`, or a fetch, in the CEO's checkout. `relinkSubmodules` now reads the paths through the same `submoduleNames`.
- [x] The fallback is never recursive: with local paths allowed, a nested `.gitmodules` could name any repo on the machine. Nested submodules then go the usual way.
- [x] tests: the URL forms; a private submodule filled from the CEO's checkout; one the CEO hasn't checked out (the error); an agent's `file://` URL never fetched (the CEO's copy is used); a nested local URL refused after a fallback (checked to fail without the guard); `TestSyncFetchesOnlyOverHTTPS` now removes the CEO's copy first, so the refusal it checks still applies.
- [x] specs: [Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules), [Protocol → `set_submodule`](../product/architecture/protocol.md#set_submodule)
- [ ] the CEO restarts Sol (or switches), and `strategy-doc/platform-doc` fills: it needs `platform-doc` checked out in the CEO's own `strategy-doc` clone
- `set_submodule(url)` still accepts only https: an agent adding a submodule still can't name an SSH URL. Left as it is (not asked).
